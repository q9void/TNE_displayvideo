package inbound

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/thenexusengine/tne_springwire/agentic"
	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// GetMutations is the inbound RTBExtensionPoint RPC handler.
//
// Per the ARTF v1.0 wire model, the caller submits an RTBRequest carrying
// their BidRequest (or BidResponse), Originator, lifecycle, and a list of
// applicable_intents they're willing to accept back. The callee returns
// an RTBResponse with mutations the callee suggests applying.
//
// In Phase 1, Catalyst is the *caller* — we dial extension-point agents
// and apply what they return. In Phase 2A, Catalyst additionally exposes
// the *callee* role: a buyer/curator agent who treats us as an extension
// point can dial us, and we respond with mutations we'd suggest.
//
// Phase 2A scope: Catalyst has no DSP-side intelligence to suggest
// mutations FROM the SSP perspective, so this handler returns an empty
// mutation list with structured metadata documenting the no-op. The
// surface is still real and useful — it proves AAMP wire compatibility,
// exercises auth + rate limit + idempotency + telemetry, and gives
// curators a concrete endpoint to integrate against. Phase 2B's
// OpenDirect is where curator-submitted deals turn into revenue.
//
// Behavioral contracts (PRD §5.1):
//   - Auth via mTLS (Phase 2A.1) or DevAuth (Phase 2A.0)
//   - Originator type ∈ {DSP, PUBLISHER}; SSP rejected (R5.1.4)
//   - Rate limited per-agent + per-publisher
//   - Idempotency on RTBRequest.id (60s window, R5.1.11)
//   - Caller's applicable_intents intersected with our outbound capability
//     set; intersection logged for observability
//   - Defensive panic recovery; auction (any third-party) never fails
//     because of this handler (R5.1.6)
//   - 4 MiB max recv (R5.1.9, enforced via grpc.MaxRecvMsgSize)
func (s *Server) GetMutations(ctx context.Context, req *pb.RTBRequest) (rsp *pb.RTBResponse, retErr error) {
	startedAt := time.Now()

	// R5.1.6: defensive panic recovery.
	defer func() {
		if r := recover(); r != nil {
			s.metrics.PanicRecoveredTotal.Inc()
			logger.Log.Error().
				Interface("panic", r).
				Str("evt", "agentic.inbound.panic").
				Str("auction_id", req.GetId()).
				Msg("inbound.handler.panic recovered")
			rsp = nil
			retErr = status.Error(codes.Internal, "internal handler error")
		}
	}()

	// 1. Authentication.
	identity, err := s.auth.Verify(ctx)
	if err != nil {
		s.metrics.AuthFailedTotal.WithLabelValues("unknown", classifyAuthErr(err)).Inc()
		s.recordCallStat(req, identity, "auth_failed", time.Since(startedAt))
		return nil, status.Error(codes.Unauthenticated, "auth failed")
	}

	// 2. Originator type gate (R5.1.4): only TYPE_DSP or TYPE_PUBLISHER may
	// act as a caller of our inbound RTBExtensionPoint. TYPE_SSP is reserved
	// for our own outbound stamping.
	if req.Originator != nil && req.Originator.Type != nil {
		t := *req.Originator.Type
		if t != pb.Originator_TYPE_DSP && t != pb.Originator_TYPE_PUBLISHER {
			s.recordCallStat(req, identity, "originator_rejected", time.Since(startedAt))
			return nil, status.Error(codes.PermissionDenied,
				"only TYPE_DSP / TYPE_PUBLISHER may submit inbound RTBExtensionPoint calls")
		}
	}

	// 3. Per-agent rate limit + circuit breaker.
	if err := s.rate.AllowAgent(identity.AgentID); err != nil {
		s.recordCallStat(req, identity, "rate_limited", time.Since(startedAt))
		if errors.Is(err, ErrCircuitOpen) {
			return nil, status.Error(codes.Unavailable, "circuit_open")
		}
		return nil, status.Error(codes.ResourceExhausted, "per-agent QPS exceeded")
	}

	// 4. Per-publisher rate limit. Publisher ID is conventionally pulled
	// from BidRequest.site.publisher.id / app.publisher.id; Phase 2A skips
	// this check when the inbound RTBRequest has no BidRequest attached.
	if pubID := publisherIDFrom(req); pubID != "" {
		if err := s.rate.AllowPublisher(pubID); err != nil {
			s.recordCallStat(req, identity, "rate_limited_pub", time.Since(startedAt))
			return nil, status.Error(codes.ResourceExhausted, "per-publisher QPS exceeded")
		}
	}

	// 5. Idempotency cache (R5.1.11).
	if cached := s.idempotencyLookup(req.GetId()); cached != nil {
		s.metrics.IdempotencyCacheHits.Inc()
		s.recordCallStat(req, identity, "idempotent_hit", time.Since(startedAt))
		return cached, nil
	}

	// 6. Build the response. Phase 2A is a no-op responder: we have no
	// SSP-side mutations to suggest. Future phases (or specialised seller
	// behaviors like proactive segment exposure) will populate Mutations.
	lc := agentic.LifecycleFromProto(req.GetLifecycle())
	apiVer := "1.0"
	mv := buildVersionStamp()
	rsp = &pb.RTBResponse{
		Id:        proto.String(req.GetId()),
		Mutations: nil, // Phase 2A: no SSP-suggested mutations
		Metadata: &pb.Metadata{
			ApiVersion:   &apiVer,
			ModelVersion: &mv,
		},
	}

	// 7. Stash the inbound applicable_intents intersection with our
	// advertised capability for observability — useful for curator-side
	// debugging without exposing internal logic.
	intents := intersectIntents(req.GetApplicableIntents(), s.registry.Capabilities().IntentsOutbound)

	// 8. Cache for idempotency.
	s.idempotencyStore(req.GetId(), rsp)

	// 9. Per-call telemetry.
	s.rate.RecordSuccess(identity.AgentID)
	s.recordCallStatWithIntents(req, identity, "ok", time.Since(startedAt), intents, lc)
	return rsp, nil
}

// recordCallStat emits the per-call structured log line + Prom histogram.
func (s *Server) recordCallStat(req *pb.RTBRequest, identity *AgentIdentity, statusStr string, dur time.Duration) {
	s.recordCallStatWithIntents(req, identity, statusStr, dur, nil, agentic.LifecycleFromProto(req.GetLifecycle()))
}

// recordCallStatWithIntents extends the basic call stat with applicable-
// intents observability for the success path.
func (s *Server) recordCallStatWithIntents(req *pb.RTBRequest, identity *AgentIdentity, statusStr string, dur time.Duration, intents []string, lc agentic.Lifecycle) {
	agentID := "unknown"
	if identity != nil {
		agentID = identity.AgentID
	}
	s.metrics.CallDuration.WithLabelValues(agentID, lc.String(), statusStr).Observe(dur.Seconds())

	ev := logger.Log.Info().
		Str("evt", "agentic.inbound.call").
		Str("direction", "inbound").
		Str("caller_agent_id", agentID).
		Str("auction_id", req.GetId()).
		Str("lifecycle", lc.String()).
		Str("status", statusStr).
		Int64("latency_ms", dur.Milliseconds())
	if len(intents) > 0 {
		ev = ev.Strs("intents_intersection", intents)
	}
	ev.Msg("inbound call")
}

// publisherIDFrom extracts a publisher_id from the inbound RTBRequest if
// the BidRequest carries one. Phase 2A treats inbound calls without a
// publisher context as not-rate-limited per-publisher; that's safe because
// the per-agent cap still applies.
func publisherIDFrom(req *pb.RTBRequest) string {
	if req == nil || req.BidRequest == nil {
		return ""
	}
	br := req.BidRequest
	if site := br.GetSite(); site != nil && site.GetPublisher() != nil {
		return site.GetPublisher().GetId()
	}
	if app := br.GetApp(); app != nil && app.GetPublisher() != nil {
		return app.GetPublisher().GetId()
	}
	return ""
}

// intersectIntents returns the set of caller-applicable intents we
// advertise as outbound-capable. Empty slice if either side empty.
func intersectIntents(callerIntents []pb.Intent, outboundAdvertised []string) []string {
	if len(callerIntents) == 0 || len(outboundAdvertised) == 0 {
		return nil
	}
	advertised := map[string]bool{}
	for _, s := range outboundAdvertised {
		advertised[strings.ToUpper(s)] = true
	}
	out := make([]string, 0, len(callerIntents))
	for _, i := range callerIntents {
		// proto enum names are like "ACTIVATE_SEGMENTS"; trim any
		// "INTENT_" prefix (TrimPrefix is a no-op when absent).
		name := strings.TrimPrefix(strings.ToUpper(i.String()), "INTENT_")
		if advertised[name] {
			out = append(out, name)
		}
	}
	return out
}

// buildVersionStamp returns a short build identifier emitted on every
// RTBResponse so buyer-agents can correlate behavioral changes with
// our deploys. Phase 2A returns a static string; future phases can
// substitute a generated package var seeded from the build SHA.
func buildVersionStamp() string {
	return "phase-2a-noop"
}

// classifyAuthErr labels auth failures for Prom + log. Maps sentinels to
// short stage strings.
func classifyAuthErr(err error) string {
	switch {
	case errors.Is(err, ErrAuthFailedRegistry):
		return "registry"
	case errors.Is(err, ErrAuthFailedSPKI):
		return "spki"
	case errors.Is(err, ErrAuthFailedDealset):
		return "dealset"
	case errors.Is(err, ErrAuthFailed):
		return "credential"
	default:
		return "other"
	}
}

// Compile-time guard: Server satisfies the generated server interface.
var _ pb.RTBExtensionPointServer = (*Server)(nil)
