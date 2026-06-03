package recorder

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/yuhuibin/go-ai-obs/provider"
)

// testRegistry returns a fresh Prometheus registry for isolated tests.
func testRegistry() prometheus.Registerer {
	return prometheus.NewRegistry()
}

// mockProvider implements provider.AIProvider for testing.
type mockProvider struct {
	name             string
	reqAttrs         []attribute.KeyValue
	respInfo         provider.CallInfo
	costFunc         func(model string, in, out int) float64
	extractReqCalls  int
	extractRespCalls int
}

func (m *mockProvider) Name() string                              { return m.name }
func (m *mockProvider) Operation() provider.Operation             { return provider.OpChat }
func (m *mockProvider) ExtractRequest(req any) []attribute.KeyValue { m.extractReqCalls++; return m.reqAttrs }
func (m *mockProvider) ExtractResponse(resp any, err error) provider.CallInfo {
	m.extractRespCalls++
	if err != nil {
		return provider.CallInfo{Provider: m.name, FinishReason: "error"}
	}
	return m.respInfo
}
func (m *mockProvider) ExtractMessages(req, resp any) ([]provider.Message, []provider.Message) {
	return nil, nil
}
func (m *mockProvider) Cost(model string, in, out int) float64 {
	if m.costFunc != nil {
		return m.costFunc(model, in, out)
	}
	return 0.01
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		name: "mock",
		reqAttrs: []attribute.KeyValue{
			attribute.String("llm.model", "mock-model"),
		},
		respInfo: provider.CallInfo{
			Provider:     "mock",
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
			FinishReason: "stop",
		},
	}
}

func TestNewWithTracerProvider(t *testing.T) {
	tp, err := NewTestTracerProvider("test-service")
	if err != nil {
		t.Fatalf("failed to create test tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	rec := NewWithTracerProvider(Config{
		ServiceName:     "test-service",
		Environment:     "test",
		MetricsRegistry: testRegistry(),
	}, tp)

	if rec == nil {
		t.Fatal("expected non-nil recorder")
	}
	if rec.config.ServiceName != "test-service" {
		t.Errorf("expected service name 'test-service', got '%s'", rec.config.ServiceName)
	}
	if rec.metrics == nil {
		t.Error("expected non-nil metrics")
	}
}

func TestNewWithTracerProvider_Defaults(t *testing.T) {
	tp, err := NewTestTracerProvider("default-test")
	if err != nil {
		t.Fatalf("failed to create test tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	rec := NewWithTracerProvider(Config{
		MetricsRegistry: testRegistry(),
	}, tp)

	if rec.config.ServiceName != "unknown-service" {
		t.Errorf("expected default service name, got '%s'", rec.config.ServiceName)
	}
	if rec.config.SamplingRate != 1.0 {
		t.Errorf("expected default sampling rate 1.0, got %.2f", rec.config.SamplingRate)
	}
}

func TestStartCall_Basic(t *testing.T) {
	tp, err := NewTestTracerProvider("test-startcall")
	if err != nil {
		t.Fatalf("failed to create test tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	rec := NewWithTracerProvider(Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)
	mock := newMockProvider()

	ctx := context.Background()
	ctx, finish := rec.StartCall(ctx, mock, "fake-request")

	if mock.extractReqCalls != 1 {
		t.Errorf("expected 1 ExtractRequest call, got %d", mock.extractReqCalls)
	}

	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		t.Error("expected valid span in context")
	}

	finish("fake-response", nil)

	if mock.extractRespCalls != 1 {
		t.Errorf("expected 1 ExtractResponse call, got %d", mock.extractRespCalls)
	}
}

func TestStartCall_WithError(t *testing.T) {
	tp, err := NewTestTracerProvider("test-error")
	if err != nil {
		t.Fatalf("failed to create test tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	rec := NewWithTracerProvider(Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)
	mock := newMockProvider()

	ctx := context.Background()
	_, finish := rec.StartCall(ctx, mock, "fake-request")

	testErr := errors.New("api timeout")
	finish(nil, testErr)

	if mock.extractRespCalls != 1 {
		t.Errorf("expected 1 ExtractResponse call, got %d", mock.extractRespCalls)
	}
}

func TestStartCall_WithCustomAttrs(t *testing.T) {
	tp, err := NewTestTracerProvider("test-attrs")
	if err != nil {
		t.Fatalf("failed to create test tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	rec := NewWithTracerProvider(Config{
		ServiceName: "test-svc",
		CustomAttrs: []attribute.KeyValue{
			attribute.String("custom.key", "custom-value"),
		},
		Environment:     "staging",
		MetricsRegistry: testRegistry(),
	}, tp)
	mock := newMockProvider()

	ctx := context.Background()
	_, finish := rec.StartCall(ctx, mock, "fake-request")
	finish("ok", nil)

	// No panic = success
	_ = ctx
}

func TestStartCall_MultipleCalls(t *testing.T) {
	tp, err := NewTestTracerProvider("test-multi")
	if err != nil {
		t.Fatalf("failed to create test tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	rec := NewWithTracerProvider(Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)
	mock := newMockProvider()

	for i := 0; i < 5; i++ {
		ctx := context.Background()
		_, finish := rec.StartCall(ctx, mock, "req")
		finish("resp", nil)
	}

	if mock.extractReqCalls != 5 {
		t.Errorf("expected 5 ExtractRequest calls, got %d", mock.extractReqCalls)
	}
	if mock.extractRespCalls != 5 {
		t.Errorf("expected 5 ExtractResponse calls, got %d", mock.extractRespCalls)
	}
}

func TestRecorder_MetricsHandler(t *testing.T) {
	tp, err := NewTestTracerProvider("test-metrics-handler")
	if err != nil {
		t.Fatalf("failed to create test tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	rec := NewWithTracerProvider(Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)
	handler := rec.MetricsHandler()
	if handler == nil {
		t.Error("expected non-nil metrics handler")
	}
	if handler.Handler() == nil {
		t.Error("expected non-nil http.Handler from metrics")
	}
}
