package endpoints

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
	"github.com/thenexusengine/tne_springwire/pkg/redis"
)

const publishersRedisKey = "tne_catalyst:publishers"

// OnboardingAdminHandler serves the comprehensive onboarding admin hub.
// It mounts at basePath (e.g. /catalyst/admin) and handles:
//
//	GET  /                  → tabbed admin UI (server-side data injection)
//	POST /sites             → create account + publisher + Redis entry
//	POST /ad-slots          → create ad_slot row
//	POST /bidder-configs    → create slot_bidder_configs row
//	GET  /ads-txt           → return current ads.txt content
//	PUT  /ads-txt           → overwrite ads.txt file
//	GET  /sellers-json      → return current sellers.json content
//	PUT  /sellers-json      → overwrite sellers.json file
type OnboardingAdminHandler struct {
	store       *storage.PublisherStore
	redisClient *redis.Client
	basePath    string
	assetsDir   string
}

// NewOnboardingAdminHandler creates a handler mounted at basePath.
// assetsDir is the filesystem path to the assets directory (e.g. "./assets" or "/app/assets").
func NewOnboardingAdminHandler(store *storage.PublisherStore, redisClient *redis.Client, basePath, assetsDir string) *OnboardingAdminHandler {
	return &OnboardingAdminHandler{
		store:       store,
		redisClient: redisClient,
		basePath:    strings.TrimSuffix(basePath, "/"),
		assetsDir:   assetsDir,
	}
}

func (h *OnboardingAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, h.basePath)
	if sub == "" {
		sub = "/"
	}
	switch {
	case sub == "/" || sub == "":
		if r.Method == http.MethodGet {
			h.serveUI(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/sites":
		if r.Method == http.MethodPost {
			h.createSite(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/ad-slots":
		if r.Method == http.MethodPost {
			h.createAdSlot(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/bidder-configs":
		if r.Method == http.MethodPost {
			h.createBidderConfig(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/ads-txt":
		switch r.Method {
		case http.MethodGet:
			h.getAdsTxt(w, r)
		case http.MethodPut:
			h.putAdsTxt(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/sellers-json":
		switch r.Method {
		case http.MethodGet:
			h.getSellersJSON(w, r)
		case http.MethodPut:
			h.putSellersJSON(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	// Also handle PUT /configs/{id} for SSP param edits (same as ssp_admin, duplicated here so
	// the page's JS can use __API_BASE__/configs/ID without routing to a separate handler)
	case strings.HasPrefix(sub, "/configs/"):
		if r.Method == http.MethodPut {
			h.updateBidderParams(w, r, strings.TrimPrefix(sub, "/configs/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(sub, "/sites/"):
		if r.Method == http.MethodPut {
			h.updateSite(w, r, strings.TrimPrefix(sub, "/sites/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(sub, "/ad-slots/"):
		if r.Method == http.MethodPut {
			h.updateAdSlot(w, r, strings.TrimPrefix(sub, "/ad-slots/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(sub, "/bidders/"):
		if r.Method == http.MethodPut {
			h.updateBidder(w, r, strings.TrimPrefix(sub, "/bidders/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case sub == "/account-defaults":
		if r.Method == http.MethodGet {
			h.getAccountDefaults(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(sub, "/account-defaults/"):
		if r.Method == http.MethodPut {
			h.upsertAccountDefault(w, r, strings.TrimPrefix(sub, "/account-defaults/"))
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

// ─── UI ─────────────────────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) serveUI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accounts, err := h.store.GetAllAccountsWithPublishers(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("onboarding admin: failed to load accounts")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if accounts == nil {
		accounts = []storage.AccountRow{}
	}

	adSlots, err := h.store.GetAllAdSlots(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("onboarding admin: failed to load ad slots")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if adSlots == nil {
		adSlots = []storage.AdSlotRow{}
	}

	configs, err := h.store.GetAllSlotBidderConfigs(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("onboarding admin: failed to load bidder configs")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if configs == nil {
		configs = []storage.SlotBidderConfigRow{}
	}

	bidders, err := h.store.GetAllBidders(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("onboarding admin: failed to load bidders")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if bidders == nil {
		bidders = []storage.BidderRow{}
	}

	accountDefaults, err := h.store.GetAllAccountBidderDefaults(ctx)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("onboarding admin: failed to load account bidder defaults")
		accountDefaults = []storage.AccountBidderDefault{}
	}
	if accountDefaults == nil {
		accountDefaults = []storage.AccountBidderDefault{}
	}

	accountsJSON, _ := json.Marshal(accounts)
	adSlotsJSON, _ := json.Marshal(adSlots)
	configsJSON, _ := json.Marshal(configs)
	biddersJSON, _ := json.Marshal(bidders)
	accountDefaultsJSON, _ := json.Marshal(accountDefaults)

	adsTxtContent := h.readAsset("ads.txt")
	sellersJSONContent := h.readAsset("sellers.json")
	adsTxtEsc, _ := json.Marshal(adsTxtContent)
	sellersJSONEsc, _ := json.Marshal(sellersJSONContent)

	user := os.Getenv("ADMIN_USER")
	pass := os.Getenv("ADMIN_PASSWORD")
	authHeader := ""
	if user != "" && pass != "" {
		authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	}

	page := onboardingHTML
	page = strings.ReplaceAll(page, `"__ACCOUNTS__"`, string(accountsJSON))
	page = strings.ReplaceAll(page, `"__AD_SLOTS__"`, string(adSlotsJSON))
	page = strings.ReplaceAll(page, `"__SSP_CONFIGS__"`, string(configsJSON))
	page = strings.ReplaceAll(page, `"__BIDDERS__"`, string(biddersJSON))
	page = strings.ReplaceAll(page, `"__ACCOUNT_DEFAULTS__"`, string(accountDefaultsJSON))
	page = strings.ReplaceAll(page, `"__ADS_TXT__"`, string(adsTxtEsc))
	page = strings.ReplaceAll(page, `"__SELLERS_JSON__"`, string(sellersJSONEsc))
	page = strings.ReplaceAll(page, "__AUTH_HEADER__", authHeader)
	page = strings.ReplaceAll(page, "__API_BASE__", h.basePath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(page))
}

// ─── Sites ───────────────────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) createSite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body struct {
		AccountID      string `json:"account_id"`
		AccountName    string `json:"account_name"`
		Domain         string `json:"domain"`
		PublisherName  string `json:"publisher_name"`
		AllowedDomains string `json:"allowed_domains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if body.AccountID == "" || body.Domain == "" {
		http.Error(w, "account_id and domain are required", http.StatusBadRequest)
		return
	}

	accountDBID, err := h.store.CreateAccountIfNotExists(ctx, body.AccountID, body.AccountName)
	if err != nil {
		logger.Log.Error().Err(err).Msg("createSite: upsert account failed")
		http.Error(w, "Failed to create account", http.StatusInternalServerError)
		return
	}

	pubDBID, err := h.store.CreatePublisher(ctx, accountDBID, body.Domain, body.PublisherName)
	if err != nil {
		logger.Log.Error().Err(err).Msg("createSite: upsert publisher failed")
		http.Error(w, "Failed to create publisher", http.StatusInternalServerError)
		return
	}

	// Register in Redis for runtime auth
	if h.redisClient != nil && body.AllowedDomains != "" {
		if err := h.redisClient.HSet(ctx, publishersRedisKey, body.AccountID, body.AllowedDomains); err != nil {
			logger.Log.Warn().Err(err).Str("account_id", body.AccountID).Msg("createSite: Redis registration failed")
			// non-fatal — log and continue
		}
	}

	logger.Log.Info().
		Str("account_id", body.AccountID).
		Str("domain", body.Domain).
		Int("publisher_db_id", pubDBID).
		Msg("New site created via admin")

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"account_db_id":%d,"publisher_db_id":%d}`, accountDBID, pubDBID)
}

// ─── Ad Slots ────────────────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) createAdSlot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body struct {
		PublisherDBID int    `json:"publisher_db_id"`
		SlotPattern   string `json:"slot_pattern"`
		SlotName      string `json:"slot_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if body.PublisherDBID <= 0 || body.SlotPattern == "" {
		http.Error(w, "publisher_db_id and slot_pattern are required", http.StatusBadRequest)
		return
	}
	if body.SlotName == "" {
		body.SlotName = body.SlotPattern
	}

	id, err := h.store.CreateAdSlot(ctx, body.PublisherDBID, body.SlotPattern, body.SlotName)
	if err != nil {
		logger.Log.Error().Err(err).Msg("createAdSlot: failed")
		http.Error(w, "Failed to create ad slot", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"id":%d}`, id)
}

// ─── Bidder Configs ──────────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) createBidderConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body struct {
		AdSlotID     int             `json:"ad_slot_id"`
		BidderDBID   int             `json:"bidder_db_id"`
		DeviceType   string          `json:"device_type"`
		BidderParams json.RawMessage `json:"bidder_params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if body.AdSlotID <= 0 || body.BidderDBID <= 0 {
		http.Error(w, "ad_slot_id and bidder_db_id are required", http.StatusBadRequest)
		return
	}
	if body.DeviceType == "" {
		body.DeviceType = "all"
	}
	if !json.Valid(body.BidderParams) {
		http.Error(w, "bidder_params must be valid JSON", http.StatusBadRequest)
		return
	}

	if err := h.store.CreateSlotBidderConfig(ctx, body.AdSlotID, body.BidderDBID, body.DeviceType, body.BidderParams); err != nil {
		logger.Log.Error().Err(err).Msg("createBidderConfig: failed")
		http.Error(w, "Failed to create bidder config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── SSP param edit (mirrors ssp_admin.go updateConfig) ──────────────────────

func (h *OnboardingAdminHandler) updateBidderParams(w http.ResponseWriter, r *http.Request, idStr string) {
	ctx := r.Context()
	id := 0
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		http.Error(w, "Invalid config ID", http.StatusBadRequest)
		return
	}
	var body struct {
		BidderParams json.RawMessage `json:"bidder_params"`
		AdSlotID     int             `json:"ad_slot_id"`
		BidderDBID   int             `json:"bidder_db_id"`
		DeviceType   string          `json:"device_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if !json.Valid(body.BidderParams) {
		http.Error(w, "bidder_params must be valid JSON", http.StatusBadRequest)
		return
	}
	var err error
	if body.AdSlotID > 0 && body.BidderDBID > 0 {
		dt := body.DeviceType
		if dt == "" {
			dt = "all"
		}
		err = h.store.UpdateSlotBidderConfigFull(ctx, id, body.AdSlotID, body.BidderDBID, dt, body.BidderParams)
	} else {
		err = h.store.UpdateSlotBidderParams(ctx, id, body.BidderParams)
	}
	if err != nil {
		logger.Log.Error().Err(err).Int("id", id).Msg("updateBidderParams: failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (h *OnboardingAdminHandler) updateSite(w http.ResponseWriter, r *http.Request, idStr string) {
	id := 0
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		http.Error(w, "Invalid site ID", http.StatusBadRequest)
		return
	}
	var body struct {
		Domain string `json:"domain"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if body.Domain == "" {
		http.Error(w, "domain is required", http.StatusBadRequest)
		return
	}
	if body.Status == "" {
		body.Status = "active"
	}
	if err := h.store.UpdatePublisher(r.Context(), id, body.Domain, body.Name, body.Status); err != nil {
		logger.Log.Error().Err(err).Int("id", id).Msg("updateSite: failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (h *OnboardingAdminHandler) updateAdSlot(w http.ResponseWriter, r *http.Request, idStr string) {
	id := 0
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		http.Error(w, "Invalid ad slot ID", http.StatusBadRequest)
		return
	}
	var body struct {
		SlotPattern string `json:"slot_pattern"`
		SlotName    string `json:"slot_name"`
		Status      string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if body.SlotPattern == "" {
		http.Error(w, "slot_pattern is required", http.StatusBadRequest)
		return
	}
	if body.SlotName == "" {
		body.SlotName = body.SlotPattern
	}
	if body.Status == "" {
		body.Status = "active"
	}
	if err := h.store.UpdateAdSlot(r.Context(), id, body.SlotPattern, body.SlotName, body.Status); err != nil {
		logger.Log.Error().Err(err).Int("id", id).Msg("updateAdSlot: failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (h *OnboardingAdminHandler) updateBidder(w http.ResponseWriter, r *http.Request, idStr string) {
	id := 0
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		http.Error(w, "Invalid bidder ID", http.StatusBadRequest)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateBidderName(r.Context(), id, body.Name); err != nil {
		logger.Log.Error().Err(err).Int("id", id).Msg("updateBidder: failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── Account Bidder Defaults ─────────────────────────────────────────────────

func (h *OnboardingAdminHandler) getAccountDefaults(w http.ResponseWriter, r *http.Request) {
	defaults, err := h.store.GetAllAccountBidderDefaults(r.Context())
	if err != nil {
		logger.Log.Error().Err(err).Msg("getAccountDefaults: failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if defaults == nil {
		defaults = []storage.AccountBidderDefault{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(defaults)
}

// upsertAccountDefault handles PUT /account-defaults/{account_db_id}/{bidder_db_id}
func (h *OnboardingAdminHandler) upsertAccountDefault(w http.ResponseWriter, r *http.Request, ids string) {
	var accountDBID, bidderDBID int
	if _, err := fmt.Sscanf(ids, "%d/%d", &accountDBID, &bidderDBID); err != nil || accountDBID <= 0 || bidderDBID <= 0 {
		http.Error(w, "Invalid IDs — expected {account_db_id}/{bidder_db_id}", http.StatusBadRequest)
		return
	}
	var body struct {
		BaseParams json.RawMessage `json:"base_params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !json.Valid(body.BaseParams) {
		http.Error(w, "base_params must be valid JSON", http.StatusBadRequest)
		return
	}
	if err := h.store.UpsertAccountBidderDefault(r.Context(), accountDBID, bidderDBID, body.BaseParams); err != nil {
		logger.Log.Error().Err(err).Msg("upsertAccountDefault: failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── ads.txt ─────────────────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) getAdsTxt(w http.ResponseWriter, r *http.Request) {
	content := h.readAsset("ads.txt")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(content))
}

func (h *OnboardingAdminHandler) putAdsTxt(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.assetsDir, "ads.txt")
	if err := os.WriteFile(path, body, 0644); err != nil {
		logger.Log.Error().Err(err).Str("path", path).Msg("Failed to write ads.txt")
		http.Error(w, "Failed to write ads.txt", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── sellers.json ────────────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) getSellersJSON(w http.ResponseWriter, r *http.Request) {
	content := h.readAsset("sellers.json")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(content))
}

func (h *OnboardingAdminHandler) putSellersJSON(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20)) // 2 MB limit
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "Body must be valid JSON", http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.assetsDir, "sellers.json")
	if err := os.WriteFile(path, body, 0644); err != nil {
		logger.Log.Error().Err(err).Str("path", path).Msg("Failed to write sellers.json")
		http.Error(w, "Failed to write sellers.json", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func (h *OnboardingAdminHandler) readAsset(name string) string {
	data, err := os.ReadFile(filepath.Join(h.assetsDir, name))
	if err != nil {
		return ""
	}
	return string(data)
}

// ─── HTML ─────────────────────────────────────────────────────────────────────

const onboardingHTML = `<!DOCTYPE html>
<html lang="en" class="h-full">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Catalyst Admin</title>
<script src="https://cdn.tailwindcss.com"></script>
<style>
  [v-cloak]{display:none}
  html,body{height:100%}
  .card    { background:#111827; border:1px solid rgba(255,255,255,0.1); border-radius:0.75rem; }
  .th      { padding:0.625rem 1rem; text-align:left; font-size:0.75rem; font-weight:600; color:#6b7280; text-transform:uppercase; letter-spacing:0.05em; }
  .td      { padding:0.75rem 1rem; font-size:0.875rem; }
  .field   { display:flex; flex-direction:column; gap:0.375rem; }
  .field label { font-size:0.75rem; font-weight:500; color:#9ca3af; }
  .field-input { background:#030712; border:1px solid rgba(255,255,255,0.1); color:#f3f4f6; font-size:0.875rem; padding:0.5rem 0.75rem; border-radius:0.5rem; outline:none; transition:border-color 0.15s; width:100%; box-sizing:border-box; }
  .field-input:focus { border-color:#6366f1; }
  .filter-select { background:#111827; border:1px solid rgba(255,255,255,0.1); color:#d1d5db; font-size:0.75rem; padding:0.375rem 0.625rem; border-radius:0.5rem; outline:none; }
  .btn-primary  { display:inline-flex; align-items:center; gap:0.375rem; padding:0.5rem 0.875rem; background:#4f46e5; color:#fff; font-size:0.875rem; font-weight:500; border-radius:0.5rem; border:none; cursor:pointer; transition:background 0.15s; }
  .btn-primary:hover  { background:#4338ca; }
  .btn-primary:disabled { opacity:0.5; cursor:not-allowed; }
  .btn-success  { display:inline-flex; align-items:center; padding:0.5rem 0.875rem; background:#047857; color:#fff; font-size:0.875rem; font-weight:500; border-radius:0.5rem; border:none; cursor:pointer; transition:background 0.15s; }
  .btn-success:hover  { background:#065f46; }
  .btn-success:disabled { opacity:0.5; cursor:not-allowed; }
  .btn-ghost    { display:inline-flex; align-items:center; padding:0.5rem 0.875rem; background:rgba(255,255,255,0.05); color:#d1d5db; font-size:0.875rem; font-weight:500; border-radius:0.5rem; border:none; cursor:pointer; transition:background 0.15s; }
  .btn-ghost:hover { background:rgba(255,255,255,0.1); }
  .badge-device { display:inline-flex; padding:0.125rem 0.5rem; border-radius:0.375rem; font-size:0.75rem; font-weight:500; background:#1e1b4b; color:#93c5fd; border:1px solid rgba(59,130,246,0.3); }
</style>
</head>
<body class="bg-gray-950 text-gray-100 h-full overflow-hidden">
<div id="app" class="flex h-full" v-cloak>

  <!-- Sidebar -->
  <nav class="w-56 flex-shrink-0 bg-gray-900 border-r border-white/10 flex flex-col">
    <div class="px-5 py-4 border-b border-white/10">
      <div class="text-sm font-bold text-white">NXS Catalyst</div>
      <div class="text-xs text-gray-500 mt-0.5">Admin</div>
    </div>
    <div class="flex-1 py-3 px-2 space-y-0.5">
      <button v-for="item in nav" :key="item.id"
        @click="section = item.id"
        :class="['flex items-center w-full px-3 py-2 rounded-lg text-sm transition-colors text-left',
          section === item.id ? 'bg-indigo-600 text-white font-medium' : 'text-gray-400 hover:bg-white/5 hover:text-gray-200']">
        {{ item.label }}
      </button>
    </div>
  </nav>

  <!-- Main -->
  <div class="flex-1 flex flex-col min-w-0 overflow-hidden">

    <!-- Header -->
    <header class="flex-shrink-0 border-b border-white/10 px-6 py-3.5 flex items-center justify-between bg-gray-900/50">
      <div class="text-sm font-semibold text-white">{{ currentNav.label }}</div>
      <button v-if="currentNav.addModal" @click="openModal(currentNav.addModal)" class="btn-primary text-xs">
        + {{ currentNav.addLabel }}
      </button>
    </header>

    <!-- Content area -->
    <div class="flex-1 overflow-auto p-6">

      <!-- SITES -->
      <div v-show="section === 'sites'">
        <div class="card overflow-hidden">
          <table class="w-full">
            <thead class="border-b border-white/10 bg-gray-900/60">
              <tr>
                <th class="th">NXS Account ID</th>
                <th class="th">Account Name</th>
                <th class="th">Domain</th>
                <th class="th">Publisher Name</th>
                <th class="th">Status</th>
                <th class="th"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/5">
              <tr v-if="!flatPublishers.length">
                <td colspan="6" class="td text-center text-gray-600 py-10">No sites yet — use + Add Site above</td>
              </tr>
              <tr v-for="row in flatPublishers" :key="row.account_id + '/' + row.domain">
                <td class="td font-mono text-indigo-400 font-semibold">{{ row.account_id }}</td>
                <td class="td text-gray-300">{{ row.account_name }}</td>
                <td class="td font-mono text-blue-400">{{ row.domain }}</td>
                <td class="td text-gray-300">{{ row.publisher_name }}</td>
                <td class="td"><span :class="statusBadge(row.status)">{{ row.status }}</span></td>
                <td class="td text-right">
                  <div v-if="row.id" class="flex items-center justify-end gap-1">
                    <button @click="openEditSite(row)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">Edit</button>
                    <button @click="duplicateSite(row)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">Duplicate</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- AD UNITS -->
      <div v-show="section === 'adunits'">
        <div class="flex items-center gap-3 mb-4 flex-wrap">
          <select v-model="slotFilters.account" class="filter-select">
            <option value="">All Accounts</option>
            <option v-for="v in slotUniqueAccounts" :key="v" :value="v">{{ v }}</option>
          </select>
          <select v-model="slotFilters.domain" class="filter-select">
            <option value="">All Domains</option>
            <option v-for="v in slotUniqueDomains" :key="v" :value="v">{{ v }}</option>
          </select>
          <input v-model="slotFilters.search" type="text" placeholder="Search slot pattern..." class="field-input text-xs py-1.5 w-44">
          <span class="text-xs text-gray-600 ml-auto">{{ filteredSlots.length }} of {{ adSlots.length }}</span>
        </div>
        <div class="card overflow-hidden">
          <table class="w-full">
            <thead class="border-b border-white/10 bg-gray-900/60">
              <tr>
                <th class="th">NXS Account</th>
                <th class="th">Domain</th>
                <th class="th">Slot Pattern</th>
                <th class="th">Slot Name</th>
                <th class="th">Status</th>
                <th class="th"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/5">
              <tr v-if="!filteredSlots.length">
                <td colspan="6" class="td text-center text-gray-600 py-10">No ad units match filters</td>
              </tr>
              <tr v-for="s in filteredSlots" :key="s.id">
                <td class="td font-mono text-indigo-400 font-semibold">{{ s.account_id }}</td>
                <td class="td font-mono text-blue-400">{{ s.domain }}</td>
                <td class="td font-mono text-gray-300 max-w-xs break-all">{{ s.slot_pattern }}</td>
                <td class="td text-gray-400">{{ s.slot_name }}</td>
                <td class="td"><span :class="statusBadge(s.status)">{{ s.status }}</span></td>
                <td class="td text-right">
                  <div class="flex items-center justify-end gap-1">
                    <button @click="openEditSlot(s)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">Edit</button>
                    <button @click="duplicateSlot(s)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">Duplicate</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- SSP CONFIGS -->
      <div v-show="section === 'ssps'">
        <div class="flex items-center gap-3 mb-4 flex-wrap">
          <select v-model="filters.account" class="filter-select">
            <option value="">All Accounts</option>
            <option v-for="v in uniqueAccounts" :key="v" :value="v">{{ v }}</option>
          </select>
          <select v-model="filters.domain" class="filter-select">
            <option value="">All Domains</option>
            <option v-for="v in uniqueDomains" :key="v" :value="v">{{ v }}</option>
          </select>
          <select v-model="filters.bidder" class="filter-select">
            <option value="">All Bidders</option>
            <option v-for="v in uniqueBidders" :key="v" :value="v">{{ v }}</option>
          </select>
          <select v-model="filters.status" class="filter-select">
            <option value="">All Status</option>
            <option value="active">Active</option>
            <option value="paused">Paused</option>
          </select>
          <input v-model="filters.search" type="text" placeholder="Search slot pattern..." class="field-input text-xs py-1.5 w-44">
          <span class="text-xs text-gray-600 ml-auto">{{ filteredConfigs.length }} of {{ configs.length }}</span>
        </div>
        <div class="card overflow-hidden">
          <table class="w-full">
            <thead class="border-b border-white/10 bg-gray-900/60">
              <tr>
                <th class="th">NXS ID</th>
                <th class="th">Domain</th>
                <th class="th">Slot Pattern</th>
                <th class="th">Bidder</th>
                <th class="th">Device</th>
                <th class="th">Params</th>
                <th class="th">Status</th>
                <th class="th"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/5">
              <tr v-if="!filteredConfigs.length">
                <td colspan="8" class="td text-center text-gray-600 py-10">No configs match filters</td>
              </tr>
              <tr v-for="r in filteredConfigs" :key="r.id">
                <td class="td font-mono text-indigo-400 font-semibold">{{ r.account_id }}</td>
                <td class="td font-mono text-blue-400">{{ r.domain }}</td>
                <td class="td font-mono text-gray-300 max-w-xs break-all text-xs">{{ r.slot_pattern }}</td>
                <td class="td font-mono text-yellow-400 font-semibold">{{ r.bidder_code }}</td>
                <td class="td"><span class="badge-device">{{ r.device_type }}</span></td>
                <td class="td text-xs max-w-xs">
                  <span v-for="(v, k) in r.bidder_params" :key="k" class="inline-block mr-2 mb-0.5">
                    <span class="text-gray-500">{{ k }}:</span> <span class="text-blue-300">{{ v }}</span>
                  </span>
                </td>
                <td class="td"><span :class="statusBadge(r.status)">{{ r.status }}</span></td>
                <td class="td text-right">
                  <div class="flex items-center justify-end gap-1">
                    <button @click="openEditConfig(r)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">Edit</button>
                    <button @click="duplicateConfig(r)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">Duplicate</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- BIDDERS -->
      <div v-show="section === 'bidders'">
        <div class="card overflow-hidden">
          <table class="w-full">
            <thead class="border-b border-white/10 bg-gray-900/60">
              <tr>
                <th class="th">Code</th>
                <th class="th">Name</th>
                <th class="th">Configs</th>
                <th class="th"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/5">
              <tr v-if="!bidders.length">
                <td colspan="4" class="td text-center text-gray-600 py-10">No bidders configured</td>
              </tr>
              <tr v-for="b in bidders" :key="b.id">
                <td class="td font-mono text-yellow-400 font-semibold">{{ b.code }}</td>
                <td class="td text-gray-300">{{ b.name }}</td>
                <td class="td text-gray-500 text-xs">{{ configs.filter(function(c){ return c.bidder_code === b.code; }).length }} configs</td>
                <td class="td text-right">
                  <button @click="openEditBidder(b)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">Edit</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- BIDDER DEFAULTS -->
      <div v-show="section === 'defaults'">
        <div class="text-xs text-gray-500 mb-4">Account-level SSP parameters shared across all slots. These merge with per-slot configs at auction time — slot values win on conflict.</div>
        <div v-for="acct in accounts" :key="acct.id" class="mb-6">
          <div class="text-xs font-semibold text-indigo-400 uppercase tracking-wide mb-2">{{ acct.account_id }} — {{ acct.name }}</div>
          <div class="card overflow-hidden">
            <table class="w-full">
              <thead class="border-b border-white/10 bg-gray-900/60">
                <tr>
                  <th class="th">Bidder</th>
                  <th class="th">Base Params</th>
                  <th class="th"></th>
                </tr>
              </thead>
              <tbody class="divide-y divide-white/5">
                <tr v-for="b in bidders" :key="b.id">
                  <td class="td font-mono text-yellow-400 font-semibold text-sm">{{ b.code }}</td>
                  <td class="td text-xs">
                    <span v-if="getDefault(acct.id, b.id)" class="text-gray-300">
                      <span v-for="(v,k) in getDefault(acct.id, b.id).base_params" :key="k" class="inline-block mr-2">
                        <span class="text-gray-500">{{ k }}:</span> <span class="text-blue-300">{{ v }}</span>
                      </span>
                    </span>
                    <span v-else class="text-gray-700 italic">not set</span>
                  </td>
                  <td class="td text-right">
                    <button @click="openEditDefault(acct, b)" class="btn-ghost" style="font-size:0.72rem;padding:0.2rem 0.5rem">
                      {{ getDefault(acct.id, b.id) ? 'Edit' : 'Set' }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- ADS.TXT -->
      <div v-show="section === 'adstxt'" class="max-w-3xl">
        <div class="card p-5 flex flex-col gap-4">
          <div>
            <div class="text-sm font-semibold text-white">ads.txt</div>
            <div class="text-xs text-gray-500 mt-1">Served at <code class="text-blue-400 font-mono">/ads.txt</code> &mdash; one entry per line: <code class="text-gray-400 font-mono">ssp.com, pub-id, DIRECT, tag-id</code></div>
          </div>
          <textarea v-model="adsTxt" rows="16"
            class="bg-gray-950 border border-white/10 text-gray-100 font-mono text-xs px-3 py-2.5 rounded-lg outline-none focus:border-indigo-500 resize-y leading-relaxed">
          </textarea>
          <div class="flex items-center gap-3">
            <button @click="saveAdsTxt" :disabled="adsTxtSaving" class="btn-success">
              {{ adsTxtSaving ? 'Saving...' : 'Save ads.txt' }}
            </button>
            <span v-if="adsTxtErr" class="text-red-400 text-xs">{{ adsTxtErr }}</span>
          </div>
        </div>
      </div>

      <!-- SELLERS.JSON -->
      <!-- ── Tags Tab ─────────────────────────────────────────────────────── -->
      <div v-show="section === 'tags'">
        <div class="card p-5 mb-4">
          <div class="flex flex-wrap items-end gap-3">
            <div class="field mb-0">
              <label>Account</label>
              <select v-model="exportAccount" class="field-input w-44">
                <option value="">All Accounts</option>
                <option v-for="a in accounts" :key="a.id" :value="a.account_id">{{ a.account_id }} — {{ a.name }}</option>
              </select>
            </div>
            <div class="field mb-0">
              <label>Format</label>
              <select v-model="exportFormat" class="field-input w-36">
                <option value="async">Async JS</option>
                <option value="gam">GAM Script</option>
                <option value="iframe">iFrame URL</option>
              </select>
            </div>
            <button @click="loadTags" :disabled="exportLoading" class="btn-primary">
              {{ exportLoading ? 'Loading…' : 'Load Tags' }}
            </button>
            <button v-if="exportTags.length" @click="downloadTags" class="btn-secondary">
              Download All (.txt)
            </button>
          </div>
        </div>
        <div v-if="exportTags.length" class="card overflow-hidden">
          <table class="w-full text-sm">
            <thead class="border-b border-white/10">
              <tr class="text-left text-xs text-gray-400 font-medium">
                <th class="px-4 py-3">Account</th>
                <th class="px-4 py-3">Slot</th>
                <th class="px-4 py-3">Size</th>
                <th class="px-4 py-3">Tag Preview</th>
                <th class="px-4 py-3 text-right">Copy</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, idx) in exportTags" :key="idx" class="border-t border-white/5 hover:bg-white/5">
                <td class="px-4 py-3 text-gray-300 font-mono text-xs whitespace-nowrap">{{ row.account_id }}</td>
                <td class="px-4 py-3 text-gray-300 font-mono text-xs max-w-xs truncate" :title="row.slot_pattern">{{ row.slot_pattern }}</td>
                <td class="px-4 py-3 text-gray-400 text-xs whitespace-nowrap">{{ row.width }}×{{ row.height }}</td>
                <td class="px-4 py-3 text-xs font-mono text-gray-500 max-w-sm truncate" :title="tagForRow(row)">{{ tagForRow(row).substring(0,80) }}…</td>
                <td class="px-4 py-3 text-right">
                  <button @click="copyTag(row, idx)" class="text-xs px-2 py-1 rounded bg-indigo-600 hover:bg-indigo-500 text-white transition-colors">
                    {{ exportCopied === idx ? 'Copied!' : 'Copy' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else-if="!exportLoading" class="text-gray-500 text-sm text-center py-12">
          Select an account and click Load Tags to generate export tags.
        </div>
      </div>

      <div v-show="section === 'sellers'" class="max-w-3xl">
        <div class="card p-5 flex flex-col gap-4">
          <div>
            <div class="text-sm font-semibold text-white">sellers.json</div>
            <div class="text-xs text-gray-500 mt-1">Served at <code class="text-blue-400 font-mono">/sellers.json</code> &mdash; must be valid JSON</div>
          </div>
          <textarea v-model="sellersJson" rows="16"
            class="bg-gray-950 border border-white/10 text-gray-100 font-mono text-xs px-3 py-2.5 rounded-lg outline-none focus:border-indigo-500 resize-y leading-relaxed">
          </textarea>
          <div class="flex items-center gap-3">
            <button @click="saveSellersJson" :disabled="sellersSaving" class="btn-success">
              {{ sellersSaving ? 'Saving...' : 'Save sellers.json' }}
            </button>
            <span v-if="sellersErr" class="text-red-400 text-xs">{{ sellersErr }}</span>
          </div>
        </div>
      </div>

    </div><!-- /content area -->
  </div><!-- /main -->

  <!-- MODAL OVERLAY -->
  <div v-if="modal" @click.self="closeModal"
    class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">

    <!-- Add Site -->
    <div v-if="modal === 'add-site'" class="card w-full max-w-lg mx-4 p-6">
      <div class="flex items-center justify-between mb-5">
        <h2 class="text-sm font-semibold text-white">Add Site</h2>
        <button @click="closeModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="grid grid-cols-2 gap-4 mb-4">
        <div class="field col-span-2"><label>NXS Account *</label>
          <select v-model="form._account_pick" @change="onAccountPick" class="field-input">
            <option value="">— select existing —</option>
            <option v-for="a in accounts" :key="a.id" :value="a.account_id">{{ a.account_id }} — {{ a.name }}</option>
            <option value="__new__">+ New Account</option>
          </select></div>
        <div v-if="form._account_pick === '__new__'" class="field"><label>New NXS Account ID *</label>
          <input v-model="form.account_id" class="field-input" placeholder="NXS003"></div>
        <div v-if="form._account_pick === '__new__'" class="field"><label>Account Name</label>
          <input v-model="form.account_name" class="field-input" placeholder="Publisher Co."></div>
        <div class="field"><label>Domain *</label>
          <input v-model="form.domain" class="field-input" placeholder="example.com"></div>
        <div class="field"><label>Site Name</label>
          <input v-model="form.publisher_name" class="field-input" placeholder="Example Site"></div>
      </div>
      <div class="field mb-5"><label>Allowed Domains (pipe-separated) *</label>
        <input v-model="form.allowed_domains" class="field-input" placeholder="example.com|*.example.com"></div>
      <div class="flex items-center gap-3">
        <button @click="submitSite" :disabled="formSaving" class="btn-primary">{{ formSaving ? 'Creating...' : 'Create Site' }}</button>
        <button @click="closeModal" class="btn-ghost">Cancel</button>
        <span v-if="formErr" class="text-red-400 text-xs">{{ formErr }}</span>
      </div>
    </div>

    <!-- Add Ad Unit -->
    <div v-if="modal === 'add-adunit'" class="card w-full max-w-lg mx-4 p-6">
      <div class="flex items-center justify-between mb-5">
        <h2 class="text-sm font-semibold text-white">Add Ad Unit</h2>
        <button @click="closeModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="grid grid-cols-2 gap-4 mb-5">
        <div class="field col-span-2"><label>Publisher *</label>
          <select v-model="form.publisher_db_id" class="field-input">
            <option value="">— select —</option>
            <option v-for="opt in publisherOptions" :key="opt.id" :value="opt.id">{{ opt.label }}</option>
          </select></div>
        <div class="field"><label>Slot Pattern *</label>
          <input v-model="form.slot_pattern" class="field-input" placeholder="example.com/billboard"></div>
        <div class="field"><label>Slot Name</label>
          <input v-model="form.slot_name" class="field-input" placeholder="billboard"></div>
      </div>
      <div class="flex items-center gap-3">
        <button @click="submitSlot" :disabled="formSaving" class="btn-primary">{{ formSaving ? 'Creating...' : 'Create Ad Unit' }}</button>
        <button @click="closeModal" class="btn-ghost">Cancel</button>
        <span v-if="formErr" class="text-red-400 text-xs">{{ formErr }}</span>
      </div>
    </div>

    <!-- Add Bidder Config -->
    <div v-if="modal === 'add-config'" class="card w-full max-w-xl mx-4 p-6" style="max-height:90vh;overflow-y:auto">
      <div class="flex items-center justify-between mb-5">
        <h2 class="text-sm font-semibold text-white">Add Bidder Config</h2>
        <button @click="closeModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="grid grid-cols-2 gap-4 mb-4">
        <div class="field col-span-2"><label>Ad Slot *</label>
          <select v-model="form.ad_slot_id" class="field-input">
            <option value="">— select —</option>
            <option v-for="s in adSlots" :key="s.id" :value="s.id">{{ s.domain }} / {{ s.slot_pattern }}</option>
          </select></div>
        <div class="field"><label>Bidder *</label>
          <select v-model="form.bidder_db_id" @change="onBidderPick('form')" class="field-input">
            <option value="">— select —</option>
            <option v-for="b in bidders" :key="b.id" :value="b.id">{{ b.code }}{{ b.name ? ' (' + b.name + ')' : '' }}</option>
          </select></div>
        <div class="field"><label>Device Type</label>
          <select v-model="form.device_type" class="field-input">
            <option value="all">all</option>
            <option value="desktop">desktop</option>
            <option value="mobile">mobile</option>
          </select></div>
      </div>
      <!-- Schema-driven param fields -->
      <div v-if="formSchemaFields.length" class="mb-4">
        <div class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">Bidder Parameters</div>
        <div v-if="formDefaultsPreview" class="text-xs text-indigo-400 mb-3 bg-indigo-950/40 border border-indigo-900/40 rounded-lg px-3 py-2">
          Account defaults pre-filled: {{ formDefaultsPreview }}
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div v-for="f in formSchemaFields" :key="f.key"
            :class="['field', f.type === 'object' ? 'col-span-2' : '']">
            <label>{{ f.label }}<span v-if="f.required" class="text-red-400 ml-0.5">*</span>
              <span v-if="f.isDefault" class="text-indigo-400 ml-1 font-normal">(account default)</span>
            </label>
            <textarea v-if="f.type === 'object'" v-model="form._params[f.key]"
              rows="2" class="field-input font-mono text-xs resize-y" :placeholder="f.placeholder"></textarea>
            <input v-else-if="f.type === 'boolean'" type="checkbox" v-model="form._params[f.key]"
              class="mt-1 h-4 w-4 rounded border-white/20 bg-gray-950 text-indigo-600">
            <input v-else v-model="form._params[f.key]" class="field-input"
              :placeholder="f.placeholder" :class="f.isDefault ? 'opacity-60' : ''">
          </div>
        </div>
      </div>
      <div v-else-if="form.bidder_db_id" class="field mb-4"><label>Bidder Params (JSON) *</label>
        <textarea v-model="form.bidder_params_raw" rows="4" class="field-input font-mono text-xs resize-y"
          placeholder='{"publisherId":"12345"}'></textarea></div>
      <div class="flex items-center gap-3">
        <button @click="submitSSP" :disabled="formSaving" class="btn-primary">{{ formSaving ? 'Creating...' : 'Create Config' }}</button>
        <button @click="closeModal" class="btn-ghost">Cancel</button>
        <span v-if="formErr" class="text-red-400 text-xs">{{ formErr }}</span>
      </div>
    </div>

  </div><!-- /add modal -->

  <!-- EDIT MODAL OVERLAY -->
  <div v-if="editModal" @click.self="closeEditModal"
    class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">

    <!-- Edit Site -->
    <div v-if="editModal === 'edit-site'" class="card w-full max-w-lg mx-4 p-6">
      <div class="flex items-center justify-between mb-5">
        <h2 class="text-sm font-semibold text-white">Edit Site</h2>
        <button @click="closeEditModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="grid grid-cols-2 gap-4 mb-5">
        <div class="field"><label>Domain *</label>
          <input v-model="editForm.domain" class="field-input" placeholder="example.com"></div>
        <div class="field"><label>Publisher Name</label>
          <input v-model="editForm.publisher_name" class="field-input" placeholder="Example Site"></div>
        <div class="field"><label>Status</label>
          <select v-model="editForm.status" class="field-input">
            <option value="active">active</option>
            <option value="paused">paused</option>
            <option value="inactive">inactive</option>
          </select></div>
      </div>
      <div class="flex items-center gap-3">
        <button @click="saveEditSite" :disabled="editSaving" class="btn-success">{{ editSaving ? 'Saving...' : 'Save Changes' }}</button>
        <button @click="closeEditModal" class="btn-ghost">Cancel</button>
        <span v-if="editErr" class="text-red-400 text-xs">{{ editErr }}</span>
      </div>
    </div>

    <!-- Edit Ad Unit -->
    <div v-if="editModal === 'edit-adunit'" class="card w-full max-w-lg mx-4 p-6">
      <div class="flex items-center justify-between mb-5">
        <h2 class="text-sm font-semibold text-white">Edit Ad Unit</h2>
        <button @click="closeEditModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="grid grid-cols-2 gap-4 mb-5">
        <div class="field"><label>Slot Pattern *</label>
          <input v-model="editForm.slot_pattern" class="field-input" placeholder="example.com/billboard"></div>
        <div class="field"><label>Slot Name</label>
          <input v-model="editForm.slot_name" class="field-input" placeholder="billboard"></div>
        <div class="field"><label>Status</label>
          <select v-model="editForm.status" class="field-input">
            <option value="active">active</option>
            <option value="paused">paused</option>
            <option value="inactive">inactive</option>
          </select></div>
      </div>
      <div class="flex items-center gap-3">
        <button @click="saveEditSlot" :disabled="editSaving" class="btn-success">{{ editSaving ? 'Saving...' : 'Save Changes' }}</button>
        <button @click="closeEditModal" class="btn-ghost">Cancel</button>
        <span v-if="editErr" class="text-red-400 text-xs">{{ editErr }}</span>
      </div>
    </div>

    <!-- Edit SSP Config -->
    <div v-if="editModal === 'edit-config'" class="card w-full max-w-xl mx-4 p-6" style="max-height:90vh;overflow-y:auto">
      <div class="flex items-center justify-between mb-5">
        <h2 class="text-sm font-semibold text-white">Edit SSP Config</h2>
        <button @click="closeEditModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="grid grid-cols-2 gap-4 mb-4">
        <div class="field col-span-2"><label>Ad Slot *</label>
          <select v-model="editForm.ad_slot_id" class="field-input">
            <option value="">— select —</option>
            <option v-for="s in adSlots" :key="s.id" :value="s.id">{{ s.domain }} / {{ s.slot_pattern }}</option>
          </select></div>
        <div class="field"><label>Bidder *</label>
          <select v-model="editForm.bidder_db_id" @change="onBidderPick('editForm')" class="field-input">
            <option value="">— select —</option>
            <option v-for="b in bidders" :key="b.id" :value="b.id">{{ b.code }}{{ b.name ? ' (' + b.name + ')' : '' }}</option>
          </select></div>
        <div class="field"><label>Device Type</label>
          <select v-model="editForm.device_type" class="field-input">
            <option value="all">all</option>
            <option value="desktop">desktop</option>
            <option value="mobile">mobile</option>
          </select></div>
      </div>
      <!-- Schema-driven param fields -->
      <div v-if="editSchemaFields.length" class="mb-4">
        <div class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">Bidder Parameters</div>
        <div class="grid grid-cols-2 gap-3">
          <div v-for="f in editSchemaFields" :key="f.key"
            :class="['field', f.type === 'object' ? 'col-span-2' : '']">
            <label>{{ f.label }}<span v-if="f.required" class="text-red-400 ml-0.5">*</span>
              <span v-if="f.isDefault" class="text-indigo-400 ml-1 font-normal">(account default)</span>
            </label>
            <textarea v-if="f.type === 'object'" v-model="editForm._params[f.key]"
              rows="2" class="field-input font-mono text-xs resize-y"></textarea>
            <input v-else-if="f.type === 'boolean'" type="checkbox" v-model="editForm._params[f.key]"
              class="mt-1 h-4 w-4 rounded border-white/20 bg-gray-950 text-indigo-600">
            <input v-else v-model="editForm._params[f.key]" class="field-input"
              :placeholder="f.placeholder" :class="f.isDefault ? 'opacity-60' : ''">
          </div>
        </div>
      </div>
      <div v-else class="field mb-4"><label>Bidder Params (JSON) *</label>
        <textarea v-model="editForm.bidder_params_raw" rows="5" class="field-input font-mono text-xs resize-y"></textarea></div>
      <div class="flex items-center gap-3">
        <button @click="saveEditConfig" :disabled="editSaving" class="btn-success">{{ editSaving ? 'Saving...' : 'Save Changes' }}</button>
        <button @click="closeEditModal" class="btn-ghost">Cancel</button>
        <span v-if="editErr" class="text-red-400 text-xs">{{ editErr }}</span>
      </div>
    </div>

    <!-- Edit Bidder -->
    <div v-if="editModal === 'edit-bidder'" class="card w-full max-w-sm mx-4 p-6">
      <div class="flex items-center justify-between mb-5">
        <h2 class="text-sm font-semibold text-white">Edit Bidder — <span class="font-mono text-yellow-400">{{ editForm.code }}</span></h2>
        <button @click="closeEditModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="field mb-5"><label>Display Name</label>
        <input v-model="editForm.name" class="field-input" placeholder="Bidder display name"></div>
      <div class="flex items-center gap-3">
        <button @click="saveEditBidder" :disabled="editSaving" class="btn-success">{{ editSaving ? 'Saving...' : 'Save Changes' }}</button>
        <button @click="closeEditModal" class="btn-ghost">Cancel</button>
        <span v-if="editErr" class="text-red-400 text-xs">{{ editErr }}</span>
      </div>
    </div>

    <!-- Edit Bidder Default -->
    <div v-if="editModal === 'edit-default'" class="card w-full max-w-xl mx-4 p-6" style="max-height:90vh;overflow-y:auto">
      <div class="flex items-center justify-between mb-2">
        <h2 class="text-sm font-semibold text-white">Bidder Defaults — <span class="font-mono text-indigo-400">{{ editForm._acctID }}</span> × <span class="font-mono text-yellow-400">{{ editForm._bidderCode }}</span></h2>
        <button @click="closeEditModal" class="text-gray-500 hover:text-gray-200 text-lg leading-none">&times;</button>
      </div>
      <div class="text-xs text-gray-500 mb-4">Set account-level params shared across all slots (e.g. accountId, siteId, publisherId). Leave slot-specific fields (zoneId, adSlot, placementId) blank here.</div>
      <div v-if="defaultSchemaFields.length" class="grid grid-cols-2 gap-3 mb-5">
        <div v-for="f in defaultSchemaFields" :key="f.key"
          :class="['field', f.type === 'object' ? 'col-span-2' : '']">
          <label>{{ f.label }}<span v-if="f.required" class="text-red-400 ml-0.5">*</span></label>
          <textarea v-if="f.type === 'object'" v-model="editForm._params[f.key]"
            rows="2" class="field-input font-mono text-xs resize-y"></textarea>
          <input v-else v-model="editForm._params[f.key]" class="field-input" :placeholder="f.placeholder">
        </div>
      </div>
      <div v-else class="field mb-5"><label>Base Params (JSON)</label>
        <textarea v-model="editForm.base_params_raw" rows="5" class="field-input font-mono text-xs resize-y"></textarea></div>
      <div class="flex items-center gap-3">
        <button @click="saveEditDefault" :disabled="editSaving" class="btn-success">{{ editSaving ? 'Saving...' : 'Save Defaults' }}</button>
        <button @click="closeEditModal" class="btn-ghost">Cancel</button>
        <span v-if="editErr" class="text-red-400 text-xs">{{ editErr }}</span>
      </div>
    </div>

  </div><!-- /edit modal -->

  <!-- Toast -->
  <div v-show="toast.show"
    :class="['fixed bottom-5 right-5 px-4 py-2.5 rounded-lg text-sm font-medium z-50 border shadow-xl',
      toast.err ? 'bg-red-950 text-red-300 border-red-900' : 'bg-emerald-950 text-emerald-300 border-emerald-900']">
    {{ toast.msg }}
  </div>

</div><!-- /#app -->

<script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
<script>
Vue.createApp({
  data: function() {
    return {
      section: 'sites',
      nav: [
        { id:'sites',    label:'Sites',           addLabel:'Add Site',    addModal:'add-site'   },
        { id:'adunits',  label:'Ad Units',         addLabel:'Add Ad Unit', addModal:'add-adunit' },
        { id:'ssps',     label:'SSP Configs',      addLabel:'Add Config',  addModal:'add-config' },
        { id:'defaults', label:'Bidder Defaults',  addLabel:null,          addModal:null         },
        { id:'bidders',  label:'Bidders',          addLabel:null,          addModal:null         },
        { id:'tags',     label:'Export Tags',      addLabel:null,          addModal:null         },
        { id:'adstxt',   label:'ads.txt',          addLabel:null,          addModal:null         },
        { id:'sellers',  label:'sellers.json',     addLabel:null,          addModal:null         },
      ],
      accounts:        "__ACCOUNTS__",
      adSlots:         "__AD_SLOTS__",
      configs:         "__SSP_CONFIGS__",
      bidders:         "__BIDDERS__",
      accountDefaults: "__ACCOUNT_DEFAULTS__",
      adsTxt:          "__ADS_TXT__",
      sellersJson:     "__SELLERS_JSON__",
      filters:     { account:'', domain:'', bidder:'', status:'', search:'' },
      slotFilters: { account:'', domain:'', search:'' },
      modal:       null,
      form:        {},
      formSaving:  false,
      formErr:     '',
      editModal:   null,
      editForm:    {},
      editSaving:  false,
      editErr:     '',
      adsTxtSaving:  false,
      adsTxtErr:     '',
      sellersSaving: false,
      sellersErr:    '',
      toast:         { show:false, msg:'', err:false },
      apiBase:       '__API_BASE__',
      authHeader:    '__AUTH_HEADER__',
      exportTags:    [],
      exportFormat:  'async',
      exportAccount: '',
      exportLoading: false,
      exportCopied:  null,
    };
  },
  computed: {
    currentNav: function() {
      var s = this.section;
      return this.nav.find(function(n){ return n.id === s; }) || this.nav[0];
    },
    filteredConfigs: function() {
      var f = this.filters;
      return this.configs.filter(function(r) {
        return (!f.account || r.account_id  === f.account) &&
               (!f.domain  || r.domain      === f.domain)  &&
               (!f.bidder  || r.bidder_code === f.bidder)  &&
               (!f.status  || r.status      === f.status)  &&
               (!f.search  || r.slot_pattern.toLowerCase().indexOf(f.search.toLowerCase()) !== -1);
      });
    },
    uniqueAccounts: function() { return this.uniq(this.configs.map(function(r){ return r.account_id;  })); },
    uniqueDomains:  function() { return this.uniq(this.configs.map(function(r){ return r.domain;      })); },
    uniqueBidders:  function() { return this.uniq(this.configs.map(function(r){ return r.bidder_code; })); },
    flatPublishers: function() {
      var rows = [];
      this.accounts.forEach(function(a) {
        if (!a.publishers || !a.publishers.length) {
          rows.push({ account_id:a.account_id, account_name:a.name, domain:'—', publisher_name:'—', status:a.status });
        } else {
          a.publishers.forEach(function(p) {
            rows.push({ id:p.id, account_id:a.account_id, account_name:a.name, domain:p.domain, publisher_name:p.name||'', status:p.status });
          });
        }
      });
      return rows;
    },
    filteredSlots: function() {
      var f = this.slotFilters;
      return this.adSlots.filter(function(s) {
        return (!f.account || s.account_id === f.account) &&
               (!f.domain  || s.domain     === f.domain)  &&
               (!f.search  || s.slot_pattern.toLowerCase().indexOf(f.search.toLowerCase()) !== -1);
      });
    },
    slotUniqueAccounts: function() { return this.uniq(this.adSlots.map(function(s){ return s.account_id; })); },
    slotUniqueDomains:  function() { return this.uniq(this.adSlots.map(function(s){ return s.domain;     })); },
    publisherOptions: function() {
      var opts = [];
      this.accounts.forEach(function(a) {
        (a.publishers||[]).forEach(function(p) {
          opts.push({ id:p.id, label:a.account_id+' / '+p.domain });
        });
      });
      return opts;
    },
    formSchemaFields: function() { return this.buildSchemaFields(this.form.bidder_db_id, this.form); },
    editSchemaFields: function() { return this.buildSchemaFields(this.editForm.bidder_db_id, this.editForm); },
    defaultSchemaFields: function() { return this.buildSchemaFields(this.editForm._bidderID, this.editForm); },
    formDefaultsPreview: function() {
      if (!this.form.bidder_db_id || !this.form._acctDBID) return null;
      var d = this.getDefaultByDBIDs(this.form._acctDBID, parseInt(this.form.bidder_db_id,10));
      if (!d) return null;
      return Object.keys(d.base_params||{}).join(', ');
    },
  },
  methods: {
    // ── Schema helpers ──────────────────────────────────────────────────────
    buildSchemaFields: function(bidderDBID, formObj) {
      if (!bidderDBID) return [];
      var id = parseInt(bidderDBID, 10);
      var bidder = this.bidders.find(function(b){ return b.id === id; });
      if (!bidder || !bidder.param_schema) return [];
      var schema;
      try { schema = typeof bidder.param_schema === 'string' ? JSON.parse(bidder.param_schema) : bidder.param_schema; }
      catch(e) { return []; }
      var props = schema.properties || {};
      var required = schema.required || [];
      if (!formObj._params) formObj._params = {};
      return Object.keys(props).map(function(key) {
        var prop = props[key];
        var types = Array.isArray(prop.type) ? prop.type : [prop.type];
        var t = types[0] === 'integer' || types[0] === 'number' ? 'number'
              : types[0] === 'boolean' ? 'boolean'
              : types[0] === 'object'  ? 'object' : 'string';
        var isDefault = false;
        var placeholder = prop.description || (t === 'object' ? '{}' : '');
        return { key:key, label:key, type:t, required:required.indexOf(key)!==-1, placeholder:placeholder, isDefault:isDefault };
      });
    },
    getDefault: function(accountDBID, bidderID) {
      return this.accountDefaults.find(function(d){ return d.account_db_id===accountDBID && d.bidder_id===bidderID; }) || null;
    },
    getDefaultByDBIDs: function(accountDBID, bidderDBID) {
      return this.accountDefaults.find(function(d){ return d.account_db_id===accountDBID && d.bidder_id===bidderDBID; }) || null;
    },
    paramsFromSchemaForm: function(formObj, schemaFields) {
      var params = {};
      schemaFields.forEach(function(f) {
        var v = (formObj._params||{})[f.key];
        if (v === undefined || v === null || v === '') return;
        if (f.type === 'number') { var n = parseFloat(v); if (!isNaN(n)) params[f.key] = n; }
        else if (f.type === 'object') { try { params[f.key] = JSON.parse(v); } catch(e) { params[f.key] = v; } }
        else params[f.key] = v;
      });
      return params;
    },
    onAccountPick: function() {
      var pick = this.form._account_pick;
      if (pick && pick !== '__new__') {
        var a = this.accounts.find(function(a){ return a.account_id === pick; });
        if (a) { this.form.account_id = a.account_id; this.form.account_name = a.name; this.form._acctDBID = a.id; }
      } else {
        this.form.account_id = ''; this.form.account_name = ''; this.form._acctDBID = null;
      }
    },
    onBidderPick: function(target) {
      var formObj = target === 'form' ? this.form : this.editForm;
      if (!formObj._params) formObj._params = {};
      var id = parseInt(formObj.bidder_db_id, 10);
      // Pre-fill from account defaults if available
      var acctDBID = target === 'form' ? formObj._acctDBID : null;
      if (acctDBID) {
        var def = this.getDefaultByDBIDs(acctDBID, id);
        if (def) {
          var base = def.base_params || {};
          Object.keys(base).forEach(function(k){ formObj._params[k] = base[k]; });
        }
      }
    },
    statusBadge: function(s) {
      if (s === 'active') return 'inline-flex px-2 py-0.5 rounded-md text-xs font-medium bg-green-900/40 text-green-400 border border-green-900/60';
      if (s === 'paused') return 'inline-flex px-2 py-0.5 rounded-md text-xs font-medium bg-yellow-900/40 text-yellow-400 border border-yellow-900/60';
      return 'inline-flex px-2 py-0.5 rounded-md text-xs font-medium bg-gray-800 text-gray-400 border border-gray-700';
    },
    openModal: function(name) {
      this.modal      = name;
      this.form       = { device_type:'all' };
      this.formErr    = '';
      this.formSaving = false;
    },
    closeModal: function() {
      this.modal   = null;
      this.form    = {};
      this.formErr = '';
    },
    uniq: function(arr) {
      return arr.filter(function(v,i,a){ return a.indexOf(v)===i; }).sort();
    },
    showToast: function(msg, isErr) {
      this.toast = { show:true, msg:msg, err:!!isErr };
      var self = this;
      setTimeout(function(){ self.toast.show = false; }, 3500);
    },
    apiFetch: function(url, method, jsonBody, rawBody, contentType) {
      var headers = {};
      if (this.authHeader) headers['Authorization'] = this.authHeader;
      var body;
      if (rawBody !== undefined && rawBody !== null) {
        headers['Content-Type'] = contentType || 'text/plain';
        body = rawBody;
      } else if (jsonBody !== null && jsonBody !== undefined) {
        headers['Content-Type'] = 'application/json';
        body = JSON.stringify(jsonBody);
      }
      return fetch(url, { method:method, headers:headers, body:body });
    },

    submitSite: async function() {
      // Resolve account_id from dropdown pick
      if (this.form._account_pick && this.form._account_pick !== '__new__') {
        this.form.account_id = this.form._account_pick;
      }
      if (!this.form.account_id || !this.form.domain || !this.form.allowed_domains) {
        this.formErr = 'Account, Domain, and Allowed Domains are required';
        return;
      }
      this.formSaving = true; this.formErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/sites', 'POST', {
          account_id:      this.form.account_id,
          account_name:    this.form.account_name   || '',
          domain:          this.form.domain,
          publisher_name:  this.form.publisher_name || '',
          allowed_domains: this.form.allowed_domains,
        });
        if (!res.ok) throw new Error(await res.text());
        var data = await res.json();
        var accountID = this.form.account_id;
        var acc = this.accounts.find(function(a){ return a.account_id === accountID; });
        if (!acc) {
          acc = { id:data.account_db_id, account_id:this.form.account_id,
                  name:this.form.account_name||'', status:'active', publishers:[] };
          this.accounts.push(acc);
        }
        acc.publishers.push({ id:data.publisher_db_id, domain:this.form.domain,
          name:this.form.publisher_name||'', status:'active' });
        this.showToast('Site created: '+this.form.domain, false);
        this.closeModal();
      } catch(e) {
        this.formErr = 'Error: '+e.message;
      } finally {
        this.formSaving = false;
      }
    },

    submitSlot: async function() {
      if (!this.form.publisher_db_id || !this.form.slot_pattern) {
        this.formErr = 'Publisher and slot pattern are required';
        return;
      }
      this.formSaving = true; this.formErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/ad-slots', 'POST', {
          publisher_db_id: parseInt(this.form.publisher_db_id, 10),
          slot_pattern:    this.form.slot_pattern,
          slot_name:       this.form.slot_name || this.form.slot_pattern,
        });
        if (!res.ok) throw new Error(await res.text());
        var data = await res.json();
        var pubInfo = { account_id:'', domain:'' };
        var pid = parseInt(this.form.publisher_db_id, 10);
        this.accounts.forEach(function(a) {
          (a.publishers||[]).forEach(function(p) {
            if (p.id === pid) { pubInfo = { account_id:a.account_id, domain:p.domain }; }
          });
        });
        this.adSlots.push({ id:data.id, account_id:pubInfo.account_id, publisher_id:pid,
          domain:pubInfo.domain, slot_pattern:this.form.slot_pattern,
          slot_name:this.form.slot_name||this.form.slot_pattern, is_adhesion:false, status:'active' });
        this.showToast('Ad unit created: '+this.form.slot_pattern, false);
        this.closeModal();
      } catch(e) {
        this.formErr = 'Error: '+e.message;
      } finally {
        this.formSaving = false;
      }
    },

    closeEditModal: function() {
      this.editModal = null;
      this.editForm  = {};
      this.editErr   = '';
      this.editSaving = false;
    },

    openEditSite: function(row) {
      this.editModal = 'edit-site';
      this.editForm  = { id:row.id, domain:row.domain, publisher_name:row.publisher_name, status:row.status };
      this.editErr   = '';
      this.editSaving = false;
    },
    saveEditSite: async function() {
      if (!this.editForm.domain) { this.editErr = 'Domain is required'; return; }
      this.editSaving = true; this.editErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/sites/'+this.editForm.id, 'PUT',
          { domain:this.editForm.domain, name:this.editForm.publisher_name||'', status:this.editForm.status });
        if (!res.ok) throw new Error(await res.text());
        var id = this.editForm.id; var ef = this.editForm;
        this.accounts.forEach(function(a) {
          (a.publishers||[]).forEach(function(p) {
            if (p.id === id) { p.domain = ef.domain; p.name = ef.publisher_name||''; p.status = ef.status; }
          });
        });
        this.showToast('Site updated', false);
        this.closeEditModal();
      } catch(e) { this.editErr = 'Error: '+e.message; }
      finally { this.editSaving = false; }
    },
    duplicateSite: function(row) {
      this.form = { _account_pick:row.account_id, account_id:row.account_id, account_name:row.account_name,
        domain:'', publisher_name:row.publisher_name+' (copy)', allowed_domains:row.domain };
      this.modal = 'add-site'; this.formErr = ''; this.formSaving = false;
    },

    openEditSlot: function(s) {
      this.editModal = 'edit-adunit';
      this.editForm  = { id:s.id, slot_pattern:s.slot_pattern, slot_name:s.slot_name, status:s.status };
      this.editErr   = '';
      this.editSaving = false;
    },
    saveEditSlot: async function() {
      if (!this.editForm.slot_pattern) { this.editErr = 'Slot pattern is required'; return; }
      this.editSaving = true; this.editErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/ad-slots/'+this.editForm.id, 'PUT',
          { slot_pattern:this.editForm.slot_pattern, slot_name:this.editForm.slot_name||this.editForm.slot_pattern, status:this.editForm.status });
        if (!res.ok) throw new Error(await res.text());
        var id = this.editForm.id; var ef = this.editForm;
        var slot = this.adSlots.find(function(s){ return s.id === id; });
        if (slot) { slot.slot_pattern = ef.slot_pattern; slot.slot_name = ef.slot_name||ef.slot_pattern; slot.status = ef.status; }
        this.showToast('Ad unit updated', false);
        this.closeEditModal();
      } catch(e) { this.editErr = 'Error: '+e.message; }
      finally { this.editSaving = false; }
    },
    duplicateSlot: function(s) {
      this.form = { publisher_db_id:s.publisher_id, slot_pattern:'', slot_name:s.slot_name+' (copy)', device_type:'all' };
      this.modal = 'add-adunit'; this.formErr = ''; this.formSaving = false;
    },

    openEditConfig: function(r) {
      this.editModal = 'edit-config';
      this.editForm  = { id:r.id, ad_slot_id:r.ad_slot_id, bidder_db_id:r.bidder_id,
        device_type:r.device_type, bidder_params_raw:JSON.stringify(r.bidder_params||{}, null, 2),
        _params: JSON.parse(JSON.stringify(r.bidder_params||{})) };
      this.editErr   = '';
      this.editSaving = false;
    },
    saveEditConfig: async function() {
      var params;
      if (this.editSchemaFields.length) {
        params = this.paramsFromSchemaForm(this.editForm, this.editSchemaFields);
      } else {
        try { params = JSON.parse(this.editForm.bidder_params_raw || '{}'); }
        catch(e) { this.editErr = 'Invalid JSON: '+e.message; return; }
      }
      if (!this.editForm.ad_slot_id || !this.editForm.bidder_db_id) {
        this.editErr = 'Ad slot and bidder are required'; return;
      }
      this.editSaving = true; this.editErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/configs/'+this.editForm.id, 'PUT', {
          ad_slot_id:parseInt(this.editForm.ad_slot_id,10),
          bidder_db_id:parseInt(this.editForm.bidder_db_id,10),
          device_type:this.editForm.device_type||'all',
          bidder_params:params,
        });
        if (!res.ok) throw new Error(await res.text());
        var id = this.editForm.id; var ef = this.editForm;
        var cfg = this.configs.find(function(r){ return r.id === id; });
        if (cfg) {
          cfg.bidder_params = params;
          cfg.device_type = ef.device_type||'all';
          cfg.ad_slot_id = parseInt(ef.ad_slot_id,10);
          cfg.bidder_id = parseInt(ef.bidder_db_id,10);
          var bidder = this.bidders.find(function(b){ return b.id === parseInt(ef.bidder_db_id,10); });
          if (bidder) cfg.bidder_code = bidder.code;
          var slot = this.adSlots.find(function(s){ return s.id === parseInt(ef.ad_slot_id,10); });
          if (slot) { cfg.domain = slot.domain; cfg.slot_pattern = slot.slot_pattern; cfg.account_id = slot.account_id; }
        }
        this.showToast('Config updated', false);
        this.closeEditModal();
      } catch(e) { this.editErr = 'Error: '+e.message; }
      finally { this.editSaving = false; }
    },
    duplicateConfig: function(r) {
      this.form = { ad_slot_id:r.ad_slot_id, bidder_db_id:r.bidder_id,
        device_type:r.device_type, bidder_params_raw:JSON.stringify(r.bidder_params||{}, null, 2) };
      this.modal = 'add-config'; this.formErr = ''; this.formSaving = false;
    },

    openEditDefault: function(acct, bidder) {
      var existing = this.getDefault(acct.id, bidder.id);
      var params = existing ? JSON.parse(JSON.stringify(existing.base_params||{})) : {};
      this.editModal = 'edit-default';
      this.editForm  = { _acctDBID:acct.id, _acctID:acct.account_id, _bidderID:bidder.id,
        _bidderCode:bidder.code, _params:params, base_params_raw:JSON.stringify(params, null, 2) };
      this.editErr   = '';
      this.editSaving = false;
    },
    saveEditDefault: async function() {
      var params;
      if (this.defaultSchemaFields.length) {
        params = this.paramsFromSchemaForm(this.editForm, this.defaultSchemaFields);
      } else {
        try { params = JSON.parse(this.editForm.base_params_raw || '{}'); }
        catch(e) { this.editErr = 'Invalid JSON: '+e.message; return; }
      }
      this.editSaving = true; this.editErr = '';
      try {
        var url = this.apiBase+'/account-defaults/'+this.editForm._acctDBID+'/'+this.editForm._bidderID;
        var res = await this.apiFetch(url, 'PUT', { base_params:params });
        if (!res.ok) throw new Error(await res.text());
        // Update in-memory
        var acctID = this.editForm._acctDBID; var bdrID = this.editForm._bidderID;
        var existing = this.accountDefaults.find(function(d){ return d.account_db_id===acctID && d.bidder_id===bdrID; });
        if (existing) { existing.base_params = params; }
        else { this.accountDefaults.push({ account_db_id:acctID, account_id:this.editForm._acctID,
          bidder_id:bdrID, bidder_code:this.editForm._bidderCode, base_params:params }); }
        this.showToast('Defaults saved for '+this.editForm._bidderCode, false);
        this.closeEditModal();
      } catch(e) { this.editErr = 'Error: '+e.message; }
      finally { this.editSaving = false; }
    },
    openEditBidder: function(b) {
      this.editModal = 'edit-bidder';
      this.editForm  = { id:b.id, code:b.code, name:b.name };
      this.editErr   = '';
      this.editSaving = false;
    },
    saveEditBidder: async function() {
      this.editSaving = true; this.editErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/bidders/'+this.editForm.id, 'PUT', { name:this.editForm.name });
        if (!res.ok) throw new Error(await res.text());
        var id = this.editForm.id; var name = this.editForm.name;
        var bidder = this.bidders.find(function(b){ return b.id === id; });
        if (bidder) bidder.name = name;
        this.showToast('Bidder updated', false);
        this.closeEditModal();
      } catch(e) { this.editErr = 'Error: '+e.message; }
      finally { this.editSaving = false; }
    },
    submitSSP: async function() {
      if (!this.form.ad_slot_id || !this.form.bidder_db_id) {
        this.formErr = 'Ad slot and bidder are required';
        return;
      }
      var params;
      if (this.formSchemaFields.length) {
        params = this.paramsFromSchemaForm(this.form, this.formSchemaFields);
      } else {
        try { params = JSON.parse(this.form.bidder_params_raw || '{}'); }
        catch(e) { this.formErr = 'Invalid JSON: '+e.message; return; }
      }
      this.formSaving = true; this.formErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/bidder-configs', 'POST', {
          ad_slot_id:    parseInt(this.form.ad_slot_id,   10),
          bidder_db_id:  parseInt(this.form.bidder_db_id, 10),
          device_type:   this.form.device_type || 'all',
          bidder_params: params,
        });
        if (!res.ok) throw new Error(await res.text());
        this.showToast('Bidder config created. Reload to see in table.', false);
        this.closeModal();
      } catch(e) {
        this.formErr = 'Error: '+e.message;
      } finally {
        this.formSaving = false;
      }
    },

    saveAdsTxt: async function() {
      this.adsTxtSaving = true; this.adsTxtErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/ads-txt', 'PUT', null, this.adsTxt, 'text/plain');
        if (!res.ok) throw new Error(await res.text());
        this.showToast('ads.txt saved', false);
      } catch(e) {
        this.adsTxtErr = 'Error: '+e.message;
        this.showToast('Save failed', true);
      } finally {
        this.adsTxtSaving = false;
      }
    },

    saveSellersJson: async function() {
      try { JSON.parse(this.sellersJson); }
      catch(e) { this.sellersErr = 'Invalid JSON: '+e.message; return; }
      this.sellersSaving = true; this.sellersErr = '';
      try {
        var res = await this.apiFetch(this.apiBase+'/sellers-json', 'PUT', null, this.sellersJson, 'application/json');
        if (!res.ok) throw new Error(await res.text());
        this.showToast('sellers.json saved', false);
      } catch(e) {
        this.sellersErr = 'Error: '+e.message;
        this.showToast('Save failed', true);
      } finally {
        this.sellersSaving = false;
      }
    },
    loadTags: async function() {
      this.exportLoading = true;
      this.exportTags = [];
      try {
        var url = '/admin/adtag/export-bulk?account_id='+encodeURIComponent(this.exportAccount);
        var res = await fetch(url, { headers: { 'Authorization': this.authHeader } });
        if (!res.ok) throw new Error(await res.text());
        this.exportTags = await res.json();
      } catch(e) {
        this.showToast('Failed to load tags: '+e.message, true);
      } finally {
        this.exportLoading = false;
      }
    },
    tagForRow: function(row) {
      if (this.exportFormat === 'gam') return row.gam_script;
      if (this.exportFormat === 'iframe') return row.iframe_url;
      return row.async_tag;
    },
    copyTag: function(row, idx) {
      var tag = this.tagForRow(row);
      navigator.clipboard.writeText(tag).then(function() {});
      this.exportCopied = idx;
      var self = this;
      setTimeout(function() { self.exportCopied = null; }, 1500);
    },
    downloadTags: function() {
      var url = '/admin/adtag/export-bulk?account_id='+encodeURIComponent(this.exportAccount)+'&download=1';
      window.open(url, '_blank');
    },
  },
}).mount('#app');
</script>
</body>
</html>`
