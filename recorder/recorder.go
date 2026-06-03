package recorder

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/yuhuibin/go-ai-obs/provider"
)

// Recorder is the core observability engine. It manages span lifecycle,
// attribute recording, and metric collection for LLM calls.
type Recorder struct {
	config  Config
	tracer  trace.Tracer
	metrics *Metrics
}

// Config holds configuration for the Recorder.
type Config struct {
	ServiceName     string
	Environment     string
	CustomAttrs     []attribute.KeyValue
	SamplingRate    float64
	MetricsRegistry prometheus.Registerer // nil means prometheus.DefaultRegisterer
}

// New creates a new Recorder. Callers should call Shutdown when done.
func New(cfg Config) (*Recorder, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "unknown-service"
	}
	if cfg.SamplingRate <= 0 {
		cfg.SamplingRate = 1.0
	}

	tp, err := newTracerProvider(cfg.ServiceName, cfg.SamplingRate)
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(tp)

	reg := cfg.MetricsRegistry
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	return &Recorder{
		config: cfg,
		tracer: tp.Tracer(
			"go-ai-obs",
			trace.WithInstrumentationVersion(Version),
		),
		metrics: NewMetricsWithRegistry(cfg.ServiceName, reg),
	}, nil
}

// NewWithTracerProvider creates a Recorder with a pre-configured TracerProvider.
// Use this for testing or when you need full control over the tracing setup.
func NewWithTracerProvider(cfg Config, tp *sdktrace.TracerProvider) *Recorder {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "unknown-service"
	}
	if cfg.SamplingRate <= 0 {
		cfg.SamplingRate = 1.0
	}

	reg := cfg.MetricsRegistry
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	return &Recorder{
		config: cfg,
		tracer: tp.Tracer(
			"go-ai-obs",
			trace.WithInstrumentationVersion(Version),
		),
		metrics: NewMetricsWithRegistry(cfg.ServiceName, reg),
	}
}

// Shutdown flushes pending spans and stops the trace exporter.
func (r *Recorder) Shutdown(ctx context.Context) error {
	if tp, ok := otel.GetTracerProvider().(shutdownable); ok {
		return tp.Shutdown(ctx)
	}
	return nil
}

type shutdownable interface {
	Shutdown(ctx context.Context) error
}

// CallResult holds the outcome of a traced LLM call.
type CallResult struct {
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	Cost         float64
	Duration     time.Duration
	Error        error
	FinishReason string
}

// StartCall begins a traced LLM call span and returns a context and a finish function.
// Call the finish function with the response and error when the LLM call completes.
func (r *Recorder) StartCall(ctx context.Context, p provider.AIProvider, req any) (context.Context, func(resp any, err error)) {
	start := time.Now()

	ctx, span := r.tracer.Start(ctx, "llm.call",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("llm.provider", p.Name()),
		),
	)

	// Extract and set request attributes
	reqAttrs := p.ExtractRequest(req)
	span.SetAttributes(reqAttrs...)

	// Add custom attributes from config
	if len(r.config.CustomAttrs) > 0 {
		span.SetAttributes(r.config.CustomAttrs...)
	}
	if r.config.Environment != "" {
		span.SetAttributes(attribute.String("deployment.environment", r.config.Environment))
	}

	return ctx, func(resp any, err error) {
		duration := time.Since(start)

		info := p.ExtractResponse(resp, err)
		cost := p.Cost(info.Model, info.InputTokens, info.OutputTokens)

		// Record span attributes
		respAttrs := []attribute.KeyValue{
			attribute.String("llm.model", info.Model),
			attribute.Int("llm.usage.input_tokens", info.InputTokens),
			attribute.Int("llm.usage.output_tokens", info.OutputTokens),
			attribute.Int("llm.usage.total_tokens", info.InputTokens+info.OutputTokens),
			attribute.Float64("llm.usage.cost_dollars", cost),
			attribute.Float64("llm.duration_ms", float64(duration.Milliseconds())),
			attribute.String("llm.finish_reason", info.FinishReason),
		}
		span.SetAttributes(respAttrs...)

		// Set span status based on error
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "success")
		}

		span.End()

		// Record metrics
		status := "success"
		if err != nil {
			status = "error"
		}

		r.metrics.RecordCall(metricLabels{
			Service:  r.config.ServiceName,
			Provider: p.Name(),
			Model:    info.Model,
			Status:   status,
		}, metricValues{
			DurationSec:  duration.Seconds(),
			InputTokens:  info.InputTokens,
			OutputTokens: info.OutputTokens,
			CostDollars:  cost,
		})
	}
}

// MetricsHandler returns an HTTP handler for the /metrics endpoint.
func (r *Recorder) MetricsHandler() *Metrics {
	return r.metrics
}
