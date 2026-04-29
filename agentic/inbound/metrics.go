package inbound

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the four Prometheus metrics defined by PRD §5.5 for the
// inbound surface. Constructed once via mustRegisterMetrics; subsequent
// calls return the cached instance so multiple Server constructions in a
// single process don't duplicate-register (which Prometheus panics on).
type Metrics struct {
	CallDuration       *prometheus.HistogramVec
	MutationTotal      *prometheus.CounterVec
	AuthFailedTotal    *prometheus.CounterVec
	RegistryFailures   prometheus.Counter
	IdempotencyCacheHits prometheus.Counter
	PanicRecoveredTotal  prometheus.Counter
}

var (
	metricsOnce     sync.Once
	cachedMetrics   *Metrics
	cachedMetricsMu sync.Mutex
)

// mustRegisterMetrics returns a single shared Metrics instance, registering
// against the default Prometheus registry on first call. Safe to call
// concurrently.
func mustRegisterMetrics() *Metrics {
	metricsOnce.Do(func() {
		m := &Metrics{
			CallDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "agentic",
				Subsystem: "inbound",
				Name:      "call_duration_seconds",
				Help:      "Duration of inbound RTBExtensionPoint.GetMutations calls.",
				Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12), // 1ms .. ~4s
			}, []string{"caller_agent_id", "lifecycle", "status"}),

			MutationTotal: promauto.NewCounterVec(prometheus.CounterOpts{
				Namespace: "agentic",
				Subsystem: "inbound",
				Name:      "mutation_total",
				Help:      "Mutations processed inbound, by intent and decision.",
			}, []string{"caller_agent_id", "intent", "decision"}),

			AuthFailedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
				Namespace: "agentic",
				Subsystem: "inbound",
				Name:      "auth_failed_total",
				Help:      "Inbound authentication failures by stage.",
			}, []string{"caller_agent_id", "stage"}),

			RegistryFailures: promauto.NewCounter(prometheus.CounterOpts{
				Namespace: "agentic",
				Name:      "registry_refresh_failed_total",
				Help:      "IAB Tools Portal Registry refresh failures since start.",
			}),

			IdempotencyCacheHits: promauto.NewCounter(prometheus.CounterOpts{
				Namespace: "agentic",
				Subsystem: "inbound",
				Name:      "idempotency_cache_hits_total",
				Help:      "Inbound calls served from the per-RTBRequest.id idempotency cache.",
			}),

			PanicRecoveredTotal: promauto.NewCounter(prometheus.CounterOpts{
				Namespace: "agentic",
				Subsystem: "inbound",
				Name:      "panic_recovered_total",
				Help:      "Inbound handler panics recovered defensively.",
			}),
		}
		cachedMetricsMu.Lock()
		cachedMetrics = m
		cachedMetricsMu.Unlock()
	})
	cachedMetricsMu.Lock()
	defer cachedMetricsMu.Unlock()
	return cachedMetrics
}
