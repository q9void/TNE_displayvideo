package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/thenexusengine/tne_springwire/agentic"
	agenticEndpoints "github.com/thenexusengine/tne_springwire/agentic/endpoints"
	"github.com/thenexusengine/tne_springwire/internal/bidcache"
	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/adapters/appnexus"
	// _ "github.com/thenexusengine/tne_springwire/internal/adapters/demo" // Disabled - no demo bids in production
	"github.com/thenexusengine/tne_springwire/internal/adapters/kargo"
	"github.com/thenexusengine/tne_springwire/internal/adapters/pubmatic"
	"github.com/thenexusengine/tne_springwire/internal/adapters/rubicon"
	"github.com/thenexusengine/tne_springwire/internal/adapters/sovrn"
	"github.com/thenexusengine/tne_springwire/internal/adapters/triplelift"
	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/analytics"
	analyticsIDR "github.com/thenexusengine/tne_springwire/internal/analytics/idr"
	analyticsPG "github.com/thenexusengine/tne_springwire/internal/analytics/postgres"
	pbsconfig "github.com/thenexusengine/tne_springwire/internal/config"
	"github.com/thenexusengine/tne_springwire/internal/endpoints"
	"github.com/thenexusengine/tne_springwire/internal/exchange"
	"github.com/thenexusengine/tne_springwire/internal/metrics"
	"github.com/thenexusengine/tne_springwire/internal/middleware"
	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/internal/usersync"
	"github.com/thenexusengine/tne_springwire/pkg/currency"
	"github.com/thenexusengine/tne_springwire/pkg/idr"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
	"github.com/thenexusengine/tne_springwire/pkg/redis"
)

// Server represents the PBS server
type Server struct {
	config            *ServerConfig
	httpServer        *http.Server
	metrics           *metrics.Metrics
	exchange          *exchange.Exchange
	rateLimiter       *middleware.RateLimiter
	rawDB             *sql.DB
	db                *storage.BidderStore
	publisher         *storage.PublisherStore
	idGraphStore      *storage.IDGraphStore
	userSyncStore     *storage.UserSyncStore
	redisClient       *redis.Client
	currencyConverter *currency.Converter
	routingLoader     *routing.Loader

	// Agentic (IAB ARTF v1.0). Populated only when AGENTIC_ENABLED=true.
	agenticRegistry *agentic.Registry
	agenticClient   *agentic.Client
}

// NewServer creates a new PBS server instance
func NewServer(cfg *ServerConfig) (*Server, error) {
	s := &Server{
		config: cfg,
	}

	if err := s.initialize(); err != nil {
		return nil, err
	}

	return s, nil
}

// initialize sets up all server components
func (s *Server) initialize() error {
	log := logger.Log

	log.Info().
		Str("port", s.config.Port).
		Str("idr_url", s.config.IDRUrl).
		Bool("idr_enabled", s.config.IDREnabled).
		Dur("timeout", s.config.Timeout).
		Msg("Initializing The Nexus Engine PBS Server")

	// Initialize Prometheus metrics
	s.metrics = metrics.NewMetrics("pbs")
	log.Info().Msg("Prometheus metrics enabled")

	// Initialize database if configured
	if err := s.initDatabase(); err != nil {
		// Database failures are non-fatal, log and continue
		log.Warn().Err(err).Msg("Database initialization failed, continuing with reduced functionality")
	}

	// Initialize middleware
	s.initMiddleware()

	// Initialize exchange
	s.initExchange()

	// Initialize Redis if configured
	if err := s.initRedis(); err != nil {
		// Redis failures are non-fatal, log and continue
		log.Warn().Err(err).Msg("Redis initialization failed, continuing with reduced functionality")
	}

	// List registered bidders
	bidders := adapters.DefaultRegistry.ListBidders()
	log.Info().
		Int("count", len(bidders)).
		Strs("bidders", bidders).
		Msg("Static bidders registered")

	// Initialize handlers and build HTTP server
	s.initHandlers()

	return nil
}

// initDatabase initializes database connections
func (s *Server) initDatabase() error {
	log := logger.Log

	if s.config.DatabaseConfig == nil {
		log.Info().Msg("DB_HOST not set, database-backed features disabled")
		return nil
	}

	// Create context for database connection and operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbCfg := s.config.DatabaseConfig
	dbConn, err := storage.NewDBConnection(
		ctx,
		dbCfg.Host,
		dbCfg.Port,
		dbCfg.User,
		dbCfg.Password,
		dbCfg.Name,
		dbCfg.SSLMode,
	)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to connect to PostgreSQL, database-backed features disabled")
		return err
	}

	s.rawDB = dbConn
	s.db = storage.NewBidderStore(dbConn)
	s.publisher = storage.NewPublisherStore(dbConn)

	s.routingLoader = routing.NewLoader(s.publisher)
	kargo.SetLoader(s.routingLoader)
	sovrn.SetLoader(s.routingLoader)
	pubmatic.SetLoader(s.routingLoader)
	triplelift.SetLoader(s.routingLoader)
	appnexus.SetLoader(s.routingLoader)
	rubicon.SetLoader(s.routingLoader)

	s.idGraphStore = storage.NewIDGraphStore(dbConn)
	s.userSyncStore = storage.NewUserSyncStore(dbConn)
	log.Info().Msg("ID graph store initialized")
	log.Info().Msg("User sync store initialized")

	// Load and log bidders from database
	// Old schema table queries removed - now using new schema
	// (accounts → publishers_new → ad_slots → slot_bidder_configs)
	// Bidder and publisher data loaded on-demand via GetSlotBidderConfigs

	return nil
}

// initMiddleware initializes all middleware components
func (s *Server) initMiddleware() {
	log := logger.Log

	// Initialize PublisherAuth
	publisherAuth := middleware.NewPublisherAuth(middleware.DefaultPublisherAuthConfig())
	if publisherAuth.IsEnabled() {
		log.Info().Msg("PublisherAuth enabled for /openrtb2/auction endpoint")
	}

	// Store rate limiter for graceful shutdown
	s.rateLimiter = middleware.NewRateLimiter(middleware.DefaultRateLimitConfig())

	log.Info().Msg("Middleware initialized")
}

// initExchange initializes the exchange engine
func (s *Server) initExchange() {
	log := logger.Log

	// Initialize currency converter if enabled
	if s.config.CurrencyConversionEnabled {
		s.currencyConverter = currency.NewConverter(currency.DefaultConfig())

		// Start background rate updates
		ctx := context.Background()
		if err := s.currencyConverter.Start(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to start currency converter, currency conversion disabled")
			s.currencyConverter = nil
		} else {
			log.Info().
				Str("default_currency", s.config.DefaultCurrency).
				Msg("Currency converter initialized and started")
		}
	} else {
		log.Info().Msg("Currency conversion disabled")
	}

	// Initialize analytics modules
	var analyticsModules []analytics.Module

	// Initialize IDR analytics adapter if IDR is enabled
	if s.config.IDREnabled && s.config.IDRUrl != "" {
		// Create IDR client for analytics
		idrClient := idr.NewClient(s.config.IDRUrl, 150*time.Millisecond, s.config.IDRAPIKey)

		// Create IDR adapter with configuration
		idrAdapter := analyticsIDR.NewAdapter(idrClient, &analyticsIDR.Config{
			BufferSize:  s.config.ToExchangeConfig().EventBufferSize,
			VerboseMode: false, // Can be made configurable via env var
		})

		analyticsModules = append(analyticsModules, idrAdapter)

		log.Info().
			Str("adapter", "idr").
			Str("idr_url", s.config.IDRUrl).
			Msg("Analytics adapter enabled")
	}

	// Initialize Postgres analytics adapter if database is available
	if s.rawDB != nil {
		pgAdapter := analyticsPG.NewAdapter(s.rawDB)
		analyticsModules = append(analyticsModules, pgAdapter)
		log.Info().Str("adapter", "postgres").Msg("Analytics adapter enabled")
	}

	// Create multi-module broadcaster if any modules are enabled
	var analyticsModule analytics.Module
	if len(analyticsModules) > 0 {
		analyticsModule = analytics.NewMultiModule(analyticsModules...)
		log.Info().
			Int("adapter_count", len(analyticsModules)).
			Msg("Analytics module initialized with multi-sink broadcasting")
	}

	// Create exchange config with currency converter and analytics
	exchangeConfig := s.config.ToExchangeConfig()
	exchangeConfig.CurrencyConverter = s.currencyConverter
	exchangeConfig.Analytics = analyticsModule

	// Create exchange with default registry
	s.exchange = exchange.New(adapters.DefaultRegistry, exchangeConfig)

	// Wire up metrics for margin tracking
	s.exchange.SetMetrics(s.metrics)
	log.Info().Msg("Metrics connected to exchange for margin tracking")

	// Wire IAB ARTF agentic integration if enabled. The feature is fully
	// gated behind AGENTIC_ENABLED — when off, this branch is skipped and
	// no agentic code path runs.
	if s.config.Agentic != nil && s.config.Agentic.Enabled {
		reg, err := agentic.LoadRegistry(s.config.Agentic.AgentsPath, s.config.Agentic.SchemaPath)
		if err != nil {
			log.Fatal().Err(err).Msg("Agentic enabled but agents.json failed to load")
		}
		// Cross-check seller_id between env and document.
		if reg.SellerID() != s.config.Agentic.SellerID {
			log.Warn().
				Str("env_seller_id", s.config.Agentic.SellerID).
				Str("doc_seller_id", reg.SellerID()).
				Msg("AGENTIC_SELLER_ID does not match seller_id in agents.json")
		}
		stamper := agentic.OriginatorStamper{SellerID: s.config.Agentic.SellerID}
		client, err := agentic.NewClient(reg, agentic.ClientConfig{
			DefaultTmaxMs:           s.config.Agentic.TmaxMs,
			AuctionSafetyMs:         s.config.Agentic.AuctionSafetyMs,
			APIKey:                  s.config.Agentic.APIKey,
			PerAgentAPIKeys:         s.config.Agentic.PerAgentAPIKeys,
			CircuitFailureThreshold: s.config.Agentic.CircuitFailureThreshold,
			CircuitSuccessThreshold: s.config.Agentic.CircuitSuccessThreshold,
			CircuitTimeout:          time.Duration(s.config.Agentic.CircuitTimeoutSeconds) * time.Second,
			AllowInsecure:           s.config.Agentic.AllowInsecureGRPC,
		}, stamper)
		if err != nil {
			log.Fatal().Err(err).Msg("Agentic client failed to dial")
		}
		applier := agentic.NewApplier(agentic.ApplierConfig{
			MaxMutationsPerResponse: s.config.Agentic.MaxMutationsPerResponse,
			MaxIDsPerPayload:        s.config.Agentic.MaxIDsPerPayload,
			DisableShadeIntent:      s.config.Agentic.DisableShadeIntent,
			ShadeMinFraction:        0.5,
		})
		s.exchange.WithAgentic(client, applier, stamper)
		s.agenticRegistry = reg
		s.agenticClient = client
		log.Info().
			Int("agents_count", reg.AgentCount()).
			Str("seller_id", reg.SellerID()).
			Int("tmax_ms", s.config.Agentic.TmaxMs).
			Bool("shade_disabled", s.config.Agentic.DisableShadeIntent).
			Msg("IAB ARTF agentic integration enabled")
	}
}

// initRedis initializes Redis client
func (s *Server) initRedis() error {
	log := logger.Log

	if s.config.RedisURL == "" {
		log.Info().Msg("REDIS_URL not set, Redis-backed features disabled")
		return nil
	}

	var err error
	s.redisClient, err = redis.New(s.config.RedisURL)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to connect to Redis")
		return err
	}

	log.Info().Msg("Redis client initialized")
	return nil
}

// initHandlers initializes HTTP handlers and builds the handler chain
func (s *Server) initHandlers() {
	log := logger.Log

	// Create handlers
	auctionHandler := endpoints.NewAuctionHandler(s.exchange)
	statusHandler := endpoints.NewStatusHandler()
	biddersHandler := endpoints.NewDynamicInfoBiddersHandler(adapters.DefaultRegistry)

	// Video handlers
	videoHandler := endpoints.NewVideoHandler(s.exchange, s.config.HostURL, s.publisher)
	videoEventHandler := endpoints.NewVideoEventHandler(nil) // Analytics can be added later

	log.Info().Msg("Video handlers initialized")

	// Shared bid cache — stores winning ad markup by bid ID so /ad/gam can serve it
	bc := bidcache.New()

	// Ad tag handlers (direct publisher integration)
	adTagHandler := endpoints.NewAdTagHandler(s.exchange, bc)
	adTagGenerator := endpoints.NewAdTagGeneratorHandler(s.config.HostURL, s.publisher)

	log.Info().Msg("Ad tag handlers initialized")

	// Cookie sync handlers
	cookieSyncConfig := endpoints.DefaultCookieSyncConfig(s.config.HostURL)
	cookieSyncHandler := endpoints.NewCookieSyncHandler(cookieSyncConfig, s.userSyncStore)
	syncAwaiter := usersync.NewSyncAwaiter()
	setuidHandler := endpoints.NewSetUIDHandler(cookieSyncHandler.ListBidders(), s.idGraphStore, s.userSyncStore, syncAwaiter)
	optoutHandler := endpoints.NewOptOutHandler()

	log.Info().
		Str("host_url", s.config.HostURL).
		Int("syncers", len(cookieSyncHandler.ListBidders())).
		Msg("Cookie sync initialized")

	// Initialize privacy middleware
	privacyConfig := middleware.DefaultPrivacyConfig()
	if s.config.DisableGDPREnforcement {
		privacyConfig.EnforceGDPR = false
		log.Warn().Msg("GDPR enforcement disabled via PBS_DISABLE_GDPR_ENFORCEMENT")
	}
	privacyMiddleware := middleware.NewPrivacyMiddleware(privacyConfig)

	// Wrap auction handler with privacy middleware
	privacyProtectedAuction := privacyMiddleware(auctionHandler)

	log.Info().
		Bool("gdpr_enforcement", privacyConfig.EnforceGDPR).
		Bool("ccpa_enforcement", privacyConfig.EnforceCCPA).
		Bool("coppa_enforcement", privacyConfig.EnforceCOPPA).
		Bool("strict_mode", privacyConfig.StrictMode).
		Msg("Privacy middleware initialized")

	// Setup routes
	mux := http.NewServeMux()
	mux.Handle("/openrtb2/auction", privacyProtectedAuction)
	mux.Handle("/status", statusHandler)
	mux.Handle("/health", healthHandler())
	mux.Handle("/health/ready", readyHandler(s.redisClient, s.publisher, s.exchange, s.currencyConverter))
	mux.Handle("/info/bidders", biddersHandler)

	// Cookie sync endpoints
	mux.Handle("/cookie_sync", cookieSyncHandler)
	mux.Handle("/setuid", setuidHandler)
	mux.Handle("/optout", optoutHandler)

	// Video endpoints (protected by privacy middleware)
	mux.Handle("/video/vast", privacyMiddleware(http.HandlerFunc(videoHandler.HandleVASTRequest)))
	mux.Handle("/video/openrtb", privacyMiddleware(http.HandlerFunc(videoHandler.HandleOpenRTBVideo)))
	mux.Handle("/video/wrapper", privacyMiddleware(http.HandlerFunc(videoHandler.HandleVASTWrapper)))
	endpoints.RegisterVideoEventRoutes(mux, videoEventHandler)

	// VMAP ad pod endpoint for CTV/SSAI integration
	podHandler := endpoints.NewPodHandler(s.config.HostURL)
	mux.Handle("/video/pod", privacyMiddleware(http.HandlerFunc(podHandler.HandleVMAP)))

	log.Info().Msg("Video endpoints registered: /video/vast, /video/openrtb, /video/wrapper, /video/pod, /video/event/*")

	// Ad tag endpoints (direct publisher integration)
	mux.HandleFunc("/ad/js", adTagHandler.HandleJavaScriptAd)
	mux.HandleFunc("/ad/iframe", adTagHandler.HandleIframeAd)
	mux.HandleFunc("/ad/gam", adTagHandler.HandleGAMAd)
	mux.HandleFunc("/ad/track", adTagHandler.HandleAdTracking)

	log.Info().Msg("Ad tag endpoints registered: /ad/js, /ad/iframe, /ad/gam, /ad/track")

	// Catalyst MAI Publisher integration
	// Load bidder mapping configuration
	mappingPath := s.config.BidderMappingPath
	if mappingPath == "" {
		mappingPath = "config/bizbudding-all-bidders-mapping.json"
	}
	bidderMapping, err := endpoints.LoadBidderMapping(mappingPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", mappingPath).Msg("Failed to load bidder mapping")
	}
	log.Info().
		Str("path", mappingPath).
		Int("ad_units", len(bidderMapping.AdUnits)).
		Strs("bidders", bidderMapping.Publisher.DefaultBidders).
		Msg("Loaded bidder mapping configuration")

	catalystBidHandler := endpoints.NewCatalystBidHandler(s.exchange, bidderMapping, s.publisher, s.userSyncStore, syncAwaiter, bc)
	mux.Handle("/v1/bid", privacyMiddleware(http.HandlerFunc(catalystBidHandler.HandleBidRequest)))

	renderHandler := endpoints.NewRenderHandler(s.rawDB)
	mux.HandleFunc("/v1/render", renderHandler.HandleRenderEvent)

	log.Info().
		Bool("hierarchical_config", s.publisher != nil).
		Msg("Catalyst MAI Publisher endpoint registered: /v1/bid, /v1/render")

	// Static assets
	mux.HandleFunc("/assets/tne-ads.js", endpoints.HandleAssets)
	mux.HandleFunc("/assets/catalyst-sdk.js", endpoints.HandleCatalystSDK)

	// IAB TCF Device Storage Disclosure (GDPR compliance)
	// Standard .well-known path and convenience root path
	mux.HandleFunc("/.well-known/tcf-disclosure.json", endpoints.HandleTCFDisclosure)
	mux.HandleFunc("/tcf-disclosure.json", endpoints.HandleTCFDisclosure)

	log.Info().Msg("TCF disclosure endpoints registered: /.well-known/tcf-disclosure.json, /tcf-disclosure.json")

	// IAB Sellers.json (supply chain transparency)
	mux.HandleFunc("/sellers.json", endpoints.HandleSellersJSON)
	mux.HandleFunc("/.well-known/sellers.json", endpoints.HandleSellersJSON)

	log.Info().Msg("Sellers.json endpoints registered: /sellers.json, /.well-known/sellers.json")

	// IAB AAMP/ARTF agents.json (agentic agent discovery, PRD §5.3 / §8.1).
	// Returns 404 unless AGENTIC_ENABLED=true so external scrapers do not
	// register us as an agentic SSP just because the route exists.
	agenticEnabled := s.config.Agentic != nil && s.config.Agentic.Enabled
	agentsJSONHandler := agenticEndpoints.NewAgentsJSONHandler(s.agenticRegistry, agenticEnabled)
	mux.Handle("/agents.json", agentsJSONHandler)
	mux.Handle("/.well-known/agents.json", agentsJSONHandler)
	log.Info().Bool("enabled", agenticEnabled).Msg("Agents.json endpoints registered: /agents.json, /.well-known/agents.json")

	// Read-only agentic admin endpoints (only when enabled).
	if agenticEnabled {
		agentsAdminHandler := agenticEndpoints.NewAgentsAdminHandler(s.agenticRegistry)
		mux.Handle("/admin/agents", middleware.AdminAuth(agentsAdminHandler))
		mux.Handle("/admin/agents/", middleware.AdminAuth(agentsAdminHandler))
		log.Info().Msg("Agents admin endpoints registered: /admin/agents, /admin/agents/{id}")
	}

	// Prometheus metrics endpoint
	mux.Handle("/metrics", metrics.Handler())

	// Admin endpoints
	mux.HandleFunc("/admin/circuit-breaker", s.circuitBreakerHandler)
	mux.HandleFunc("/admin/currency", s.currencyStatsHandler)
	mux.HandleFunc("/admin/adtag/generator", adTagGenerator.HandleGeneratorUI)
	mux.HandleFunc("/admin/adtag/generate", adTagGenerator.HandleGenerateTag)
	mux.HandleFunc("/admin/adtag/export-bulk", adTagGenerator.HandleBulkExportTags)
	dashboardHandler := endpoints.NewDashboardHandler()
	metricsAPIHandler := endpoints.NewMetricsAPIHandler()
	publisherAdminHandler := endpoints.NewPublisherAdminHandler(s.redisClient)
	sspAdminHandler := endpoints.NewSSPAdminHandler(s.publisher, "/admin/ssp-ids")
	catalystAdminHandler := endpoints.NewOnboardingAdminHandler(s.publisher, s.redisClient, s.routingLoader, "/catalyst/admin", "assets")
	mux.Handle("/admin/dashboard", dashboardHandler)
	mux.Handle("/admin/metrics", metricsAPIHandler)
	mux.Handle("/admin/publishers", publisherAdminHandler)
	mux.Handle("/admin/publishers/", publisherAdminHandler)
	mux.Handle("/admin/ssp-ids", sspAdminHandler)
	mux.Handle("/admin/ssp-ids/", sspAdminHandler)
	mux.Handle("/catalyst/admin", catalystAdminHandler)
	mux.Handle("/catalyst/admin/", catalystAdminHandler)
	mux.HandleFunc("/ads.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/ads.txt")
	})

	log.Info().Msg("Admin tag generator registered: /admin/adtag/generator")
	log.Info().Msg("Onboarding admin registered: /catalyst/admin")
	log.Info().Msg("SSP ID manager registered: /admin/ssp-ids")

	// Version endpoint (similar to Prebid Server)
	mux.HandleFunc("/version", versionHandler)

	// pprof debugging endpoints (only enabled with PPROF_ENABLED=true)
	if os.Getenv("PPROF_ENABLED") == "true" {
		// Wrap pprof with admin auth middleware
		adminAuth := middleware.AdminAuth
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		mux.Handle("/debug/pprof/", adminAuth(pprofMux))
		mux.Handle("/debug/pprof/cmdline", adminAuth(http.HandlerFunc(pprof.Cmdline)))
		mux.Handle("/debug/pprof/profile", adminAuth(http.HandlerFunc(pprof.Profile)))
		mux.Handle("/debug/pprof/symbol", adminAuth(http.HandlerFunc(pprof.Symbol)))
		mux.Handle("/debug/pprof/trace", adminAuth(http.HandlerFunc(pprof.Trace)))

		log.Info().Msg("pprof debugging endpoints enabled with admin auth: /debug/pprof/*")
	} else {
		log.Info().Msg("pprof debugging endpoints disabled (set PPROF_ENABLED=true to enable)")
	}

	// Build middleware chain
	handler := s.buildHandler(mux)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      handler,
		ReadTimeout:  pbsconfig.ServerReadTimeout,
		WriteTimeout: pbsconfig.ServerWriteTimeout,
		IdleTimeout:  pbsconfig.ServerIdleTimeout,
	}
}

// buildHandler builds the middleware chain
func (s *Server) buildHandler(mux *http.ServeMux) http.Handler {
	log := logger.Log

	// Initialize middleware
	cors := middleware.NewCORS(middleware.DefaultCORSConfig())
	security := middleware.NewSecurity(nil)
	adminAuth := middleware.AdminAuth  // Admin API key authentication
	publisherAuth := middleware.NewPublisherAuth(middleware.DefaultPublisherAuthConfig())
	sizeLimiter := middleware.NewSizeLimiter(middleware.DefaultSizeLimitConfig())
	gzipMiddleware := middleware.NewGzip(middleware.DefaultGzipConfig())

	// Wire up metrics
	s.rateLimiter.SetMetrics(s.metrics)

	// Wire up stores
	if s.publisher != nil {
		publisherAuth.SetPublisherStore(s.publisher)
		log.Info().Msg("Publisher store connected to authentication middleware")
	}

	// Wire up Redis
	if s.redisClient != nil {
		publisherAuth.SetRedisClient(s.redisClient)
		log.Info().Msg("Redis client set for publisher auth middleware")
	}

	log.Info().
		Bool("cors_enabled", true).
		Bool("security_headers_enabled", security.GetConfig().Enabled).
		Bool("rate_limiting_enabled", s.rateLimiter != nil).
		Msg("Middleware chain built")

	// Build chain: CORS -> Security -> AdminAuth -> Logging -> Size Limit -> PublisherAuth -> Rate Limit -> Metrics -> Gzip -> Handler
	handler := http.Handler(mux)
	handler = gzipMiddleware.Middleware(handler)
	handler = s.metrics.Middleware(handler)
	handler = s.rateLimiter.Middleware(handler)
	handler = publisherAuth.Middleware(handler)
	handler = sizeLimiter.Middleware(handler)
	handler = loggingMiddleware(handler)
	handler = adminAuth(handler)  // Admin endpoint authentication
	handler = security.Middleware(handler)
	handler = cors.Middleware(handler)

	return handler
}

// circuitBreakerHandler returns circuit breaker stats
func (s *Server) circuitBreakerHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Log
	w.Header().Set("Content-Type", "application/json")

	response := make(map[string]interface{})

	// Include IDR circuit breaker stats
	if s.exchange.GetIDRClient() != nil {
		response["idr"] = s.exchange.GetIDRClient().CircuitBreakerStats()
	} else {
		response["idr"] = map[string]string{"status": "disabled"}
	}

	// Include bidder circuit breaker stats
	response["bidders"] = s.exchange.GetBidderCircuitBreakerStats()

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("failed to encode circuit breaker stats")
	}
}

// currencyStatsHandler returns currency converter stats
func (s *Server) currencyStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.currencyConverter == nil {
		response := map[string]interface{}{
			"status":  "disabled",
			"message": "Currency conversion is not enabled",
		}
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Log.Error().Err(err).Msg("failed to encode currency stats response")
		}
		return
	}

	stats := s.currencyConverter.Stats()
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		logger.Log.Error().Err(err).Msg("failed to encode currency stats")
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log := logger.Log
	log.Info().Str("addr", s.httpServer.Addr).Msg("Server listening")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown performs graceful shutdown
func (s *Server) Shutdown(ctx context.Context) error {
	log := logger.Log
	log.Info().Msg("Starting graceful shutdown")

	// Stop rate limiter cleanup goroutine
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}

	// Stop currency converter background refresh
	if s.currencyConverter != nil {
		s.currencyConverter.Stop()
		log.Info().Msg("Currency converter stopped")
	}

	// Flush pending events from exchange
	if s.exchange != nil {
		if err := s.exchange.Close(); err != nil {
			log.Warn().Err(err).Msg("Error flushing event recorder")
		} else {
			log.Info().Msg("Event recorder flushed")
		}
	}

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}

	log.Info().Msg("Server stopped gracefully")
	return nil
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// noLogPaths are high-frequency endpoints that don't need per-request log lines
var noLogPaths = map[string]bool{
	"/health":       true,
	"/health/ready": true,
	"/status":       true,
	"/metrics":      true,
}

// loggingMiddleware logs HTTP requests with structured logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if noLogPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Generate request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Add request ID to response
		w.Header().Set("X-Request-ID", requestID)

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log request completion
		duration := time.Since(start)

		event := logger.Log.Info()
		if wrapped.statusCode >= 400 {
			event = logger.Log.Warn()
		}
		if wrapped.statusCode >= 500 {
			event = logger.Log.Error()
		}

		event.
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.statusCode).
			Dur("duration_ms", duration).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("HTTP request")
	})
}

// healthHandler returns a simple liveness check
func healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(health); err != nil {
			logger.Log.Error().Err(err).Msg("failed to encode health response")
		}
	})
}

// sanitizeHealthCheckError returns a safe, generic error message for health check responses.
// SECURITY: Raw error messages from database/Redis may contain sensitive information such as:
// - Connection strings with hostnames, ports, or credentials
// - Internal network topology (IP addresses, service names)
// - Software version information useful for fingerprinting
// - Stack traces or internal paths
// This function logs the full error for debugging while returning only a safe message to clients.
func sanitizeHealthCheckError(service string, err error) string {
	// Log the full error for operators/debugging (internal logs only)
	logger.Log.Warn().
		Str("service", service).
		Err(err).
		Msg("Health check failed - see logs for details")

	// Return generic message to external clients
	return "connection failed"
}

// readyHandler returns a readiness check with dependency verification
// SECURITY: Error messages are sanitized to prevent information disclosure.
// Raw errors may contain connection strings, hostnames, or internal network details.
func readyHandler(redisClient *redis.Client, publisherStore *storage.PublisherStore, ex *exchange.Exchange, currencyConverter *currency.Converter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := make(map[string]interface{})
		allHealthy := true

		// Check database if available
		if publisherStore != nil {
			if err := publisherStore.Ping(ctx); err != nil {
				checks["database"] = map[string]interface{}{
					"status": "unhealthy",
					"error":  sanitizeHealthCheckError("database", err),
				}
				allHealthy = false
			} else {
				checks["database"] = map[string]interface{}{
					"status": "healthy",
				}
			}
		} else {
			checks["database"] = map[string]interface{}{
				"status": "disabled",
			}
		}

		// Check Redis if available
		if redisClient != nil {
			if err := redisClient.Ping(ctx); err != nil {
				checks["redis"] = map[string]interface{}{
					"status": "unhealthy",
					"error":  sanitizeHealthCheckError("redis", err),
				}
				allHealthy = false
			} else {
				checks["redis"] = map[string]interface{}{
					"status": "healthy",
				}
			}
		} else {
			checks["redis"] = map[string]interface{}{
				"status": "disabled",
			}
		}

		// Check IDR service if enabled
		idrClient := ex.GetIDRClient()
		if idrClient != nil {
			if err := idrClient.HealthCheck(ctx); err != nil {
				checks["idr"] = map[string]interface{}{
					"status": "unhealthy",
					"error":  sanitizeHealthCheckError("idr", err),
				}
				allHealthy = false
			} else {
				checks["idr"] = map[string]interface{}{
					"status": "healthy",
				}
			}
		} else {
			checks["idr"] = map[string]interface{}{
				"status": "disabled",
			}
		}

		// Check currency converter if enabled
		if currencyConverter != nil {
			stats := currencyConverter.Stats()
			ratesLoaded := false
			if loaded, ok := stats["ratesLoaded"].(bool); ok {
				ratesLoaded = loaded
			}

			stale := false
			if isStale, ok := stats["stale"].(bool); ok {
				stale = isStale
			}

			if !ratesLoaded || stale {
				checks["currency"] = map[string]interface{}{
					"status":      "degraded",
					"ratesLoaded": ratesLoaded,
					"stale":       stale,
				}
				// Don't mark as unhealthy - stale rates can still work
			} else {
				checks["currency"] = map[string]interface{}{
					"status":      "healthy",
					"ratesLoaded": ratesLoaded,
				}
			}
		} else {
			checks["currency"] = map[string]interface{}{
				"status": "disabled",
			}
		}

		status := http.StatusOK
		if !allHealthy {
			status = http.StatusServiceUnavailable
		}

		response := map[string]interface{}{
			"ready":     allHealthy,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"checks":    checks,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Log.Error().Err(err).Msg("failed to encode readiness response")
		}
	})
}

// versionHandler returns version information
func versionHandler(w http.ResponseWriter, r *http.Request) {
	version := map[string]string{
		"version":   "1.0.0",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(version); err != nil {
		logger.Log.Error().Err(err).Msg("failed to encode version response")
	}
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}
