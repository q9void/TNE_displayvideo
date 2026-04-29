package inbound

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	dpb "github.com/thenexusengine/tne_springwire/agentic/gen/tne/v1"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// discoveryHandler is the in-package implementation of the Discovery
// service. It's wrapped in a struct so the Server can implement the
// generated DiscoveryServer interface separately from RTBExtensionPoint
// — keeping concerns in their own files.
type discoveryHandler struct {
	dpb.UnimplementedDiscoveryServer
	server *Server
}

// registerDiscoveryHandler registers the Discovery service on the
// Server's gRPC server. Called from server.go's Start() flow.
func registerDiscoveryHandler(s *Server) {
	if s.grpcServer == nil {
		// Server.Start hasn't built the gRPC server yet; this should not
		// happen in normal flow because registerDiscoveryHandler is called
		// after grpc.NewServer(...). Defensive no-op here so a refactor of
		// Start() can't accidentally panic at boot.
		return
	}
	dpb.RegisterDiscoveryServer(s.grpcServer, &discoveryHandler{server: s})
}

// DescribeCapabilities returns the seller agent's capability manifest per
// PRD §7.3. Auth-light: requires DevAuth or mTLS verification, but does
// NOT require IAB Tools Portal Registry cross-check or per-deal authorisation.
//
// The response payload is the agentic.Capabilities block from agents.json
// serialized as JSON, plus an envelope (version + seller_id + seller_domain
// + etag). Buyer agents parse the JSON against the published agents.json
// schema; the etag lets them short-circuit re-fetches.
func (h *discoveryHandler) DescribeCapabilities(ctx context.Context, _ *dpb.DescribeCapabilitiesRequest) (*dpb.CapabilitiesResponse, error) {
	startedAt := time.Now()

	// Defensive panic recovery — Discovery is auth-light but still public-
	// facing; one bad request mustn't crash the server.
	defer func() {
		if r := recover(); r != nil {
			h.server.metrics.PanicRecoveredTotal.Inc()
			logger.Log.Error().
				Interface("panic", r).
				Str("evt", "agentic.inbound.panic").
				Str("rpc", "Discovery.DescribeCapabilities").
				Msg("inbound.discovery.panic recovered")
		}
	}()

	// Auth-light: require valid credentials but no further authorisation.
	identity, err := h.server.auth.Verify(ctx)
	if err != nil {
		h.server.metrics.AuthFailedTotal.WithLabelValues("unknown", classifyAuthErr(err)).Inc()
		return nil, status.Error(codes.Unauthenticated, "auth failed")
	}

	caps := h.server.registry.Capabilities()
	capsJSON, err := json.Marshal(caps)
	if err != nil {
		// Should never happen — Capabilities is a fixed-shape struct of
		// strings + string slices. Defensive log + 500 if it does.
		logger.Log.Error().Err(err).Msg("inbound.discovery.marshal_failed")
		return nil, status.Error(codes.Internal, "marshal failed")
	}

	rsp := &dpb.CapabilitiesResponse{
		Version:          proto.String(h.server.registry.Version()),
		SellerId:         proto.String(h.server.registry.SellerID()),
		SellerDomain:     proto.String(h.server.registry.SellerDomain()),
		CapabilitiesJson: proto.String(string(capsJSON)),
		Etag:             proto.String(computeETag(capsJSON)),
	}

	h.server.rate.RecordSuccess(identity.AgentID)
	h.server.metrics.CallDuration.WithLabelValues(identity.AgentID, "discovery", "ok").
		Observe(time.Since(startedAt).Seconds())
	logger.Log.Info().
		Str("evt", "agentic.inbound.call").
		Str("direction", "inbound").
		Str("rpc", "Discovery.DescribeCapabilities").
		Str("caller_agent_id", identity.AgentID).
		Str("etag", *rsp.Etag).
		Int64("latency_ms", time.Since(startedAt).Milliseconds()).
		Msg("inbound discovery")

	return rsp, nil
}

// computeETag returns a stable hex-encoded SHA-256 prefix over the
// capabilities JSON. Buyer agents use this to detect changes; we use the
// SHA-256 prefix (16 hex chars) so the ETag is short on the wire while
// still collision-resistant for our purposes.
func computeETag(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:8])
}

// Compile-time guard: discoveryHandler satisfies the generated server
// interface.
var _ dpb.DiscoveryServer = (*discoveryHandler)(nil)
