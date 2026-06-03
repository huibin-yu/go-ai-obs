package recorder

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewMetricsWithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry("test-svc", reg)

	if m == nil {
		t.Fatal("expected non-nil metrics")
	}
	if m.serviceName != "test-svc" {
		t.Errorf("expected service name 'test-svc', got '%s'", m.serviceName)
	}
}

func TestRecordCall(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry("test-svc", reg)

	// Record a successful call
	m.RecordCall(metricLabels{
		Service:  "test-svc",
		Provider: "openai",
		Model:    "gpt-4o",
		Status:   "success",
	}, metricValues{
		DurationSec:  1.5,
		InputTokens:  100,
		OutputTokens: 50,
		CostDollars:  0.00075,
	})

	// Record an error call
	m.RecordCall(metricLabels{
		Service:  "test-svc",
		Provider: "openai",
		Model:    "gpt-4o",
		Status:   "error",
	}, metricValues{
		DurationSec:  30.0,
		InputTokens:  0,
		OutputTokens: 0,
		CostDollars:  0.0,
	})

	// Verify request counter
	expectedReqs := `
# HELP aiobs_llm_requests_total Total number of LLM requests, partitioned by service, model, provider, and status.
# TYPE aiobs_llm_requests_total counter
aiobs_llm_requests_total{model="gpt-4o",provider="openai",service="test-svc",status="error"} 1
aiobs_llm_requests_total{model="gpt-4o",provider="openai",service="test-svc",status="success"} 1
`
	if err := testutil.CollectAndCompare(m.requestsTotal, strings.NewReader(expectedReqs)); err != nil {
		t.Errorf("requests counter mismatch: %v", err)
	}

	// Verify token counter
	expectedTokens := `
# HELP aiobs_llm_tokens_total Total number of tokens consumed, partitioned by type (input/output).
# TYPE aiobs_llm_tokens_total counter
aiobs_llm_tokens_total{model="gpt-4o",provider="openai",service="test-svc",type="input"} 100
aiobs_llm_tokens_total{model="gpt-4o",provider="openai",service="test-svc",type="output"} 50
`
	if err := testutil.CollectAndCompare(m.tokensTotal, strings.NewReader(expectedTokens)); err != nil {
		t.Errorf("tokens counter mismatch: %v", err)
	}

	// Verify cost counter
	expectedCost := `
# HELP aiobs_llm_cost_dollars_total Total estimated cost of LLM calls in dollars.
# TYPE aiobs_llm_cost_dollars_total counter
aiobs_llm_cost_dollars_total{model="gpt-4o",provider="openai",service="test-svc"} 0.00075
`
	if err := testutil.CollectAndCompare(m.costDollarsTotal, strings.NewReader(expectedCost)); err != nil {
		t.Errorf("cost counter mismatch: %v", err)
	}

	// Verify histogram (at least check observation count)
	count := testutil.CollectAndCount(m.latencySeconds)
	if count == 0 {
		t.Error("expected histogram to have observations")
	}
}

func TestRecordCall_MultipleAccumulates(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry("test-svc", reg)

	// Record 3 calls
	for i := 0; i < 3; i++ {
		m.RecordCall(metricLabels{
			Service:  "test-svc",
			Provider: "openai",
			Model:    "gpt-4o",
			Status:   "success",
		}, metricValues{
			DurationSec:  1.0,
			InputTokens:  100,
			OutputTokens: 50,
			CostDollars:  0.001,
		})
	}

	expectedReqs := `
# HELP aiobs_llm_requests_total Total number of LLM requests, partitioned by service, model, provider, and status.
# TYPE aiobs_llm_requests_total counter
aiobs_llm_requests_total{model="gpt-4o",provider="openai",service="test-svc",status="success"} 3
`
	if err := testutil.CollectAndCompare(m.requestsTotal, strings.NewReader(expectedReqs)); err != nil {
		t.Errorf("requests counter mismatch: %v", err)
	}

	expectedTokens := `
# HELP aiobs_llm_tokens_total Total number of tokens consumed, partitioned by type (input/output).
# TYPE aiobs_llm_tokens_total counter
aiobs_llm_tokens_total{model="gpt-4o",provider="openai",service="test-svc",type="input"} 300
aiobs_llm_tokens_total{model="gpt-4o",provider="openai",service="test-svc",type="output"} 150
`
	if err := testutil.CollectAndCompare(m.tokensTotal, strings.NewReader(expectedTokens)); err != nil {
		t.Errorf("tokens counter mismatch: %v", err)
	}
}
