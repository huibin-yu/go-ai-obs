package recorder

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metricLabels struct {
	Service  string
	Provider string
	Model    string
	Status   string
}

type metricValues struct {
	DurationSec  float64
	InputTokens  int
	OutputTokens int
	CostDollars  float64
}

// Metrics collects and exposes Prometheus metrics for LLM calls.
type Metrics struct {
	serviceName string
	registry    prometheus.Registerer

	requestsTotal    *prometheus.CounterVec
	tokensTotal      *prometheus.CounterVec
	latencySeconds   *prometheus.HistogramVec
	costDollarsTotal *prometheus.CounterVec
	ttftSeconds      *prometheus.HistogramVec // Time-to-First-Token
}

// NewMetrics creates and registers Prometheus metrics on the default registry.
// It panics if metrics have already been registered (use NewMetricsWithRegistry for isolated registries).
func NewMetrics(serviceName string) *Metrics {
	return NewMetricsWithRegistry(serviceName, prometheus.DefaultRegisterer)
}

// NewMetricsWithRegistry creates and registers Prometheus metrics on the given registry.
// Use this for testing or when you need an isolated registry.
func NewMetricsWithRegistry(serviceName string, reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		serviceName: serviceName,
		registry:    reg,
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aiobs",
			Name:      "llm_requests_total",
			Help:      "Total number of LLM requests, partitioned by service, model, provider, and status.",
		}, []string{"service", "model", "provider", "status"}),

		tokensTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aiobs",
			Name:      "llm_tokens_total",
			Help:      "Total number of tokens consumed, partitioned by type (input/output).",
		}, []string{"service", "model", "provider", "type"}),

		latencySeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "aiobs",
			Name:      "llm_latency_seconds",
			Help:      "Latency histogram for LLM calls in seconds.",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		}, []string{"service", "model", "provider"}),

		costDollarsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aiobs",
			Name:      "llm_cost_dollars_total",
			Help:      "Total estimated cost of LLM calls in dollars.",
		}, []string{"service", "model", "provider"}),

		ttftSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "aiobs",
			Name:      "llm_ttft_seconds",
			Help:      "Time-to-first-token for streaming LLM calls in seconds.",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}, []string{"service", "provider", "model"}),
	}

	reg.MustRegister(
		m.requestsTotal,
		m.tokensTotal,
		m.latencySeconds,
		m.costDollarsTotal,
		m.ttftSeconds,
	)

	return m
}

// RecordCall records metrics for a completed LLM call.
func (m *Metrics) RecordCall(labels metricLabels, vals metricValues) {
	l := prometheus.Labels{
		"service":  labels.Service,
		"provider": labels.Provider,
		"model":    labels.Model,
		"status":   labels.Status,
	}

	m.requestsTotal.With(l).Inc()
	m.latencySeconds.With(trimLabels(l)).Observe(vals.DurationSec)
	m.costDollarsTotal.With(trimLabels(l)).Add(vals.CostDollars)

	tokenLabels := prometheus.Labels{
		"service":  labels.Service,
		"provider": labels.Provider,
		"model":    labels.Model,
		"type":     "input",
	}
	m.tokensTotal.With(tokenLabels).Add(float64(vals.InputTokens))

	tokenLabels["type"] = "output"
	m.tokensTotal.With(tokenLabels).Add(float64(vals.OutputTokens))
}

// Handler returns an http.Handler for the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	if reg, ok := m.registry.(*prometheus.Registry); ok {
		return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	}
	return promhttp.Handler()
}

// trimLabels removes the status label for histogram/cost metrics.
func trimLabels(l prometheus.Labels) prometheus.Labels {
	return prometheus.Labels{
		"service":  l["service"],
		"provider": l["provider"],
		"model":    l["model"],
	}
}
