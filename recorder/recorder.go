package recorder

import (
	"context"
	"encoding/json"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/yuhuibin/go-ai-obs/provider"
	"github.com/yuhuibin/go-ai-obs/semconv"
)

// Recorder is the core observability engine. It manages span lifecycle,
// attribute recording, and metric collection for GenAI calls.
type Recorder struct {
	config         Config
	tracer         trace.Tracer
	metrics        *Metrics
	captureContent bool
}

// Config holds configuration for the Recorder.
type Config struct {
	ServiceName     string
	Environment     string
	CustomAttrs     []attribute.KeyValue
	SamplingRate    float64
	CaptureContent  bool // opt-in: capture gen_ai.input.messages and gen_ai.output.messages
	MetricsRegistry prometheus.Registerer
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
		config:         cfg,
		tracer:         tp.Tracer("go-ai-obs", trace.WithInstrumentationVersion(Version)),
		metrics:        NewMetricsWithRegistry(cfg.ServiceName, reg),
		captureContent: cfg.CaptureContent,
	}, nil
}

// NewWithTracerProvider creates a Recorder with a pre-configured TracerProvider.
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
		config:         cfg,
		tracer:         tp.Tracer("go-ai-obs", trace.WithInstrumentationVersion(Version)),
		metrics:        NewMetricsWithRegistry(cfg.ServiceName, reg),
		captureContent: cfg.CaptureContent,
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

// =============================================================================
// LLM Call Tracing (chat, embeddings, etc.)
// =============================================================================

// StreamMetrics holds time-to-first-token and other streaming measurements.
type StreamMetrics struct {
	TimeToFirstToken time.Duration
}

// StartCall begins a traced LLM call span using GenAI semantic conventions.
// Span name: "chat <model>" per OTel spec.
//
// Usage:
//
//	ctx, finish := rec.StartCall(ctx, provider.NewOpenAI(), req)
//	resp, err := client.CreateChatCompletion(ctx, req)
//	finish(resp, err)
func (r *Recorder) StartCall(ctx context.Context, p provider.AIProvider, req any) (context.Context, func(resp any, err error)) {
	start := time.Now()

	// Extract request attributes once, reuse for span name
	reqAttrs := p.ExtractRequest(req)
	model := modelFromAttrs(reqAttrs)

	ctx, span := r.tracer.Start(ctx, string(p.Operation())+" "+model,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(semconv.AttrOperationName, string(p.Operation())),
			attribute.String(semconv.AttrProviderName, p.Name()),
			attribute.String(semconv.AttrServiceName, r.config.ServiceName),
		),
	)

	span.SetAttributes(reqAttrs...)

	// Custom & environment attributes
	if len(r.config.CustomAttrs) > 0 {
		span.SetAttributes(r.config.CustomAttrs...)
	}
	if r.config.Environment != "" {
		span.SetAttributes(attribute.String(semconv.AttrDeploymentEnv, r.config.Environment))
	}

	return ctx, func(resp any, err error) {
		duration := time.Since(start)
		info := p.ExtractResponse(resp, err)
		cost := p.Cost(info.Model, info.InputTokens, info.OutputTokens)

		// Response attributes (GenAI standard)
		respAttrs := []attribute.KeyValue{
			attribute.String(semconv.AttrResponseModel, info.Model),
			attribute.Int(semconv.AttrUsageInputTokens, info.InputTokens),
			attribute.Int(semconv.AttrUsageOutputTokens, info.OutputTokens),
			attribute.Int(semconv.AttrUsageTotalTokens, info.InputTokens+info.OutputTokens),
			attribute.Float64(semconv.AttrUsageCostDollars, cost),
			attribute.Float64(semconv.AttrDurationMS, float64(duration.Milliseconds())),
			attribute.StringSlice(semconv.AttrResponseFinishReasons, []string{info.FinishReason}),
		}
		if info.ResponseID != "" {
			respAttrs = append(respAttrs, attribute.String(semconv.AttrResponseID, info.ResponseID))
		}
		span.SetAttributes(respAttrs...)

		// Opt-in message content capture
		if r.captureContent {
			inMsgs, outMsgs := p.ExtractMessages(req, resp)
			if len(inMsgs) > 0 {
				if b, e := json.Marshal(inMsgs); e == nil {
					span.SetAttributes(attribute.String(semconv.AttrInputMessages, string(b)))
				}
			}
			if len(outMsgs) > 0 {
				if b, e := json.Marshal(outMsgs); e == nil {
					span.SetAttributes(attribute.String(semconv.AttrOutputMessages, string(b)))
				}
			}
		}

		// Span status
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()

		// Record Prometheus metrics
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

// StartStreamCall is like StartCall but returns a StreamRecorder that can
// record TTFT (time-to-first-token) and tokens-per-second when the stream completes.
func (r *Recorder) StartStreamCall(ctx context.Context, p provider.AIProvider, req any) (context.Context, *StreamRecorder) {
	start := time.Now()
	reqAttrs := p.ExtractRequest(req)
	model := modelFromAttrs(reqAttrs)

	ctx, span := r.tracer.Start(ctx, string(p.Operation())+" "+model,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(semconv.AttrOperationName, string(p.Operation())),
			attribute.String(semconv.AttrProviderName, p.Name()),
			attribute.String(semconv.AttrServiceName, r.config.ServiceName),
		),
	)

	span.SetAttributes(reqAttrs...)

	if len(r.config.CustomAttrs) > 0 {
		span.SetAttributes(r.config.CustomAttrs...)
	}
	if r.config.Environment != "" {
		span.SetAttributes(attribute.String(semconv.AttrDeploymentEnv, r.config.Environment))
	}

	return ctx, &StreamRecorder{
		recorder: r,
		provider: p,
		span:     span,
		start:    start,
		model:    model,
	}
}

// StreamRecorder tracks streaming-specific metrics (TTFT, tokens/s).
type StreamRecorder struct {
	recorder     *Recorder
	provider     provider.AIProvider
	span         trace.Span
	start        time.Time
	model        string
	firstTokenAt time.Time
	totalTokens  int
	hasFirst     bool
}

// RecordFirstToken records the time-to-first-token. Call this when the first chunk arrives.
func (s *StreamRecorder) RecordFirstToken() {
	if !s.hasFirst {
		s.firstTokenAt = time.Now()
		s.hasFirst = true
		ttft := s.firstTokenAt.Sub(s.start)
		s.span.SetAttributes(
			attribute.Float64("gen_ai.client.operation.time_to_first_token_ms", float64(ttft.Milliseconds())),
		)
		s.recorder.metrics.ttftSeconds.With(prometheus.Labels{
			"service":  s.recorder.config.ServiceName,
			"provider": s.provider.Name(),
			"model":    s.model,
		}).Observe(ttft.Seconds())
	}
}

// AddTokens accumulates token count for throughput calculation.
func (s *StreamRecorder) AddTokens(n int) {
	s.totalTokens += n
}

// Finish completes the stream span with final metrics.
func (s *StreamRecorder) Finish(resp any, err error) {
	duration := time.Since(s.start)
	info := s.provider.ExtractResponse(resp, err)
	cost := s.provider.Cost(info.Model, info.InputTokens, info.OutputTokens)

	respAttrs := []attribute.KeyValue{
		attribute.String(semconv.AttrResponseModel, info.Model),
		attribute.Int(semconv.AttrUsageInputTokens, info.InputTokens),
		attribute.Int(semconv.AttrUsageOutputTokens, info.OutputTokens),
		attribute.Int(semconv.AttrUsageTotalTokens, info.InputTokens+info.OutputTokens),
		attribute.Float64(semconv.AttrUsageCostDollars, cost),
		attribute.Float64(semconv.AttrDurationMS, float64(duration.Milliseconds())),
		attribute.StringSlice(semconv.AttrResponseFinishReasons, []string{info.FinishReason}),
	}
	if info.ResponseID != "" {
		respAttrs = append(respAttrs, attribute.String(semconv.AttrResponseID, info.ResponseID))
	}

	// Tokens-per-second
	if duration.Seconds() > 0 && info.OutputTokens > 0 {
		tps := float64(info.OutputTokens) / duration.Seconds()
		respAttrs = append(respAttrs, attribute.Float64("gen_ai.client.tokens_per_second", tps))
	}

	s.span.SetAttributes(respAttrs...)

	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
		s.span.RecordError(err)
	} else {
		s.span.SetStatus(codes.Ok, "")
	}

	s.span.End()

	status := "success"
	if err != nil {
		status = "error"
	}

	s.recorder.metrics.RecordCall(metricLabels{
		Service:  s.recorder.config.ServiceName,
		Provider: s.provider.Name(),
		Model:    info.Model,
		Status:   status,
	}, metricValues{
		DurationSec:  duration.Seconds(),
		InputTokens:  info.InputTokens,
		OutputTokens: info.OutputTokens,
		CostDollars:  cost,
	})
}

// =============================================================================
// Agent & Tool Tracing
// =============================================================================

// AgentSpan represents an agent invocation span in a multi-step agent workflow.
// Use it to create parent spans for agent loops, with child spans for each LLM call
// and tool execution.
type AgentSpan struct {
	span  trace.Span
	start time.Time
}

// StartAgent begins an agent-level span ("invoke_agent <name>").
// All LLM calls and tool executions within this agent should use the returned context
// so they appear as child spans.
//
// Usage:
//
//	ctx, agent := rec.StartAgent(ctx, "support-bot", "v1.2")
//	defer agent.End()
//	// ... LLM calls and tool executions within ctx become children
func (r *Recorder) StartAgent(ctx context.Context, name, version string, attrs ...attribute.KeyValue) (context.Context, *AgentSpan) {
	agentAttrs := []attribute.KeyValue{
		attribute.String(semconv.AttrOperationName, semconv.OperationInvokeAgent),
		attribute.String(semconv.AttrAgentName, name),
		attribute.String(semconv.AttrServiceName, r.config.ServiceName),
	}
	if version != "" {
		agentAttrs = append(agentAttrs, attribute.String(semconv.AttrAgentVersion, version))
	}
	agentAttrs = append(agentAttrs, attrs...)

	ctx, span := r.tracer.Start(ctx, semconv.OperationInvokeAgent+" "+name,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(agentAttrs...),
	)

	if r.config.Environment != "" {
		span.SetAttributes(attribute.String(semconv.AttrDeploymentEnv, r.config.Environment))
	}

	return ctx, &AgentSpan{span: span, start: time.Now()}
}

// End completes the agent span.
func (a *AgentSpan) End() {
	a.span.SetAttributes(
		attribute.Float64(semconv.AttrDurationMS, float64(time.Since(a.start).Milliseconds())),
	)
	a.span.SetStatus(codes.Ok, "")
	a.span.End()
}

// EndWithError completes the agent span with an error.
func (a *AgentSpan) EndWithError(err error) {
	a.span.SetAttributes(
		attribute.Float64(semconv.AttrDurationMS, float64(time.Since(a.start).Milliseconds())),
	)
	a.span.SetStatus(codes.Error, err.Error())
	a.span.RecordError(err)
	a.span.End()
}

// StartTool begins a tool execution span as a child of the current context.
//
// Usage:
//
//	ctx, tool := rec.StartTool(ctx, "search_orders", "function")
//	result, err := searchOrders(ctx, query)
//	tool.End(result, err)
func (r *Recorder) StartTool(ctx context.Context, toolName, toolType string, attrs ...attribute.KeyValue) (context.Context, *ToolSpan) {
	toolAttrs := []attribute.KeyValue{
		attribute.String(semconv.AttrOperationName, semconv.OperationExecuteTool),
		attribute.String(semconv.AttrToolName, toolName),
		attribute.String(semconv.AttrToolType, toolType),
	}
	toolAttrs = append(toolAttrs, attrs...)

	ctx, span := r.tracer.Start(ctx, semconv.OperationExecuteTool+" "+toolName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(toolAttrs...),
	)

	return ctx, &ToolSpan{span: span, start: time.Now()}
}

// ToolSpan represents a tool execution span.
type ToolSpan struct {
	span  trace.Span
	start time.Time
}

// End completes the tool span with the tool result.
func (t *ToolSpan) End(result string, err error) {
	if result != "" {
		t.span.SetAttributes(attribute.String("gen_ai.tool.result", result))
	}
	t.span.SetAttributes(
		attribute.Float64(semconv.AttrDurationMS, float64(time.Since(t.start).Milliseconds())),
	)
	if err != nil {
		t.span.SetStatus(codes.Error, err.Error())
		t.span.RecordError(err)
	} else {
		t.span.SetStatus(codes.Ok, "")
	}
	t.span.End()
}

// StartChain creates a child span for a named step in an AI pipeline (e.g., "rag-retrieval").
// This is a generic span for non-LLM, non-tool steps.
func (r *Recorder) StartChain(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, func(err error)) {
	ctx, span := r.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	span.SetAttributes(attrs...)

	start := time.Now()
	return ctx, func(err error) {
		span.SetAttributes(attribute.Float64(semconv.AttrDurationMS, float64(time.Since(start).Milliseconds())))
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}
}

// =============================================================================
// Helpers
// =============================================================================

// modelFromAttrs extracts the model name from pre-extracted request attributes.
func modelFromAttrs(attrs []attribute.KeyValue) string {
	for _, attr := range attrs {
		if string(attr.Key) == semconv.AttrRequestModel {
			return attr.Value.AsString()
		}
	}
	return "unknown"
}

// MetricsHandler returns the metrics handler for the /metrics endpoint.
func (r *Recorder) MetricsHandler() *Metrics {
	return r.metrics
}
