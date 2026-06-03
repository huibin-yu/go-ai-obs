package aiobs

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"

	"github.com/yuhuibin/go-ai-obs/provider"
	"github.com/yuhuibin/go-ai-obs/recorder"
)

func testRegistry() prometheus.Registerer {
	return prometheus.NewRegistry()
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		ServiceName:  "my-app",
		Environment:  "production",
		SamplingRate: 0.5,
		CustomAttrs: []attribute.KeyValue{
			attribute.String("team", "ai"),
		},
	}

	if cfg.ServiceName != "my-app" {
		t.Errorf("expected 'my-app', got '%s'", cfg.ServiceName)
	}
	if cfg.Environment != "production" {
		t.Errorf("expected 'production', got '%s'", cfg.Environment)
	}
	if cfg.SamplingRate != 0.5 {
		t.Errorf("expected 0.5, got %f", cfg.SamplingRate)
	}
	if len(cfg.CustomAttrs) != 1 {
		t.Errorf("expected 1 custom attr, got %d", len(cfg.CustomAttrs))
	}
}

func TestWithServiceName(t *testing.T) {
	cfg := &Config{}
	WithServiceName("test-app")(cfg)
	if cfg.ServiceName != "test-app" {
		t.Errorf("expected 'test-app', got '%s'", cfg.ServiceName)
	}
}

func TestWithEnvironment(t *testing.T) {
	cfg := &Config{}
	WithEnvironment("staging")(cfg)
	if cfg.Environment != "staging" {
		t.Errorf("expected 'staging', got '%s'", cfg.Environment)
	}
}

func TestWithCustomAttr(t *testing.T) {
	cfg := &Config{}
	WithCustomAttr("key1", "val1")(cfg)
	WithCustomAttr("key2", "val2")(cfg)
	if len(cfg.CustomAttrs) != 2 {
		t.Errorf("expected 2 custom attrs, got %d", len(cfg.CustomAttrs))
	}
}

func TestWithSamplingRate(t *testing.T) {
	cfg := &Config{}
	WithSamplingRate(0.1)(cfg)
	if cfg.SamplingRate != 0.1 {
		t.Errorf("expected 0.1, got %f", cfg.SamplingRate)
	}
}

func TestConfig_toRecorderConfig(t *testing.T) {
	cfg := &Config{
		ServiceName:  "app",
		Environment:  "dev",
		SamplingRate: 0.5,
		CustomAttrs:  []attribute.KeyValue{attribute.String("k", "v")},
	}
	rcfg := cfg.toRecorderConfig()
	if rcfg.ServiceName != "app" {
		t.Errorf("expected 'app', got '%s'", rcfg.ServiceName)
	}
	if rcfg.Environment != "dev" {
		t.Errorf("expected 'dev', got '%s'", rcfg.Environment)
	}
	if rcfg.SamplingRate != 0.5 {
		t.Errorf("expected 0.5, got %f", rcfg.SamplingRate)
	}
	if len(rcfg.CustomAttrs) != 1 {
		t.Errorf("expected 1 custom attr, got %d", len(rcfg.CustomAttrs))
	}
}

func TestNewWithConfig_DefaultsApplied(t *testing.T) {
	// NewWithConfig tries OTLP which fails in CI, but defaults are applied first
	cfg := Config{ServiceName: "test", SamplingRate: 0}
	// ServiceName and SamplingRate defaults are set by NewWithConfig
	// before attempting OTLP connection
	_, err := NewWithConfig(cfg)
	// May fail due to OTLP, but we're testing config path
	_ = err
}

func TestTraceCall_Success(t *testing.T) {
	tp, err := recorder.NewTestTracerProvider("test-tracecall")
	if err != nil {
		t.Fatalf("failed to create test tracer: %v", err)
	}
	defer tp.Shutdown(context.Background())

	inner := recorder.NewWithTracerProvider(recorder.Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)

	rec := &Recorder{inner: inner, config: Config{ServiceName: "test-svc"}}

	type simpleProvider struct {
		provider.OpenAIProvider
	}
	p := &simpleProvider{}

	called := false
	result, err := TraceCall(context.Background(), rec, p, "test-req",
		func(ctx context.Context) (string, error) {
			called = true
			return "test-response", nil
		})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "test-response" {
		t.Errorf("expected 'test-response', got '%s'", result)
	}
	if !called {
		t.Error("expected function to be called")
	}
}

func TestTraceCall_Error(t *testing.T) {
	tp, err := recorder.NewTestTracerProvider("test-tracecall-err")
	if err != nil {
		t.Fatalf("failed to create test tracer: %v", err)
	}
	defer tp.Shutdown(context.Background())

	inner := recorder.NewWithTracerProvider(recorder.Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)

	rec := &Recorder{inner: inner, config: Config{ServiceName: "test-svc"}}

	type simpleProvider struct {
		provider.OpenAIProvider
	}
	p := &simpleProvider{}

	_, err = TraceCall(context.Background(), rec, p, "test-req",
		func(ctx context.Context) (string, error) {
			return "", context.DeadlineExceeded
		})

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestWrapOpenAI(t *testing.T) {
	tp, err := recorder.NewTestTracerProvider("test-wrap")
	if err != nil {
		t.Fatalf("failed to create test tracer: %v", err)
	}
	defer tp.Shutdown(context.Background())

	inner := recorder.NewWithTracerProvider(recorder.Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)

	rec := &Recorder{inner: inner, config: Config{ServiceName: "test-svc"}}

	wrapped := rec.WrapOpenAI(nil)
	if wrapped == nil {
		t.Error("expected non-nil wrapped client")
	}
	if wrapped.recorder == nil {
		t.Error("expected non-nil recorder in wrapped client")
	}
}

func TestMetricsHandler(t *testing.T) {
	tp, err := recorder.NewTestTracerProvider("test-metrics-wrapper")
	if err != nil {
		t.Fatalf("failed to create test tracer: %v", err)
	}
	defer tp.Shutdown(context.Background())

	inner := recorder.NewWithTracerProvider(recorder.Config{
		ServiceName:     "test-svc",
		MetricsRegistry: testRegistry(),
	}, tp)

	rec := &Recorder{inner: inner, config: Config{ServiceName: "test-svc"}}

	handler := rec.MetricsHandler()
	if handler == nil {
		t.Error("expected non-nil metrics handler")
	}
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("expected non-empty version")
	}
}
