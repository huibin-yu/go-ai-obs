// Package aiobs provides provider-agnostic observability for Go AI/LLM applications.
//
// It automatically traces LLM calls via OpenTelemetry (GenAI semantic conventions),
// exports Prometheus metrics, and supports multiple providers through an adapter interface.
//
// Quick start:
//
//	rec, _ := aiobs.New(aiobs.Config{ServiceName: "my-app"})
//	defer rec.Shutdown(ctx)
//
//	client := rec.WrapOpenAI(openai.NewClient("sk-..."))
//	resp, err := client.CreateChatCompletion(ctx, req)
//	// Automatically traced with gen_ai.* attributes, metrics recorded.
//
// Agent tracing:
//
//	ctx, agent := rec.StartAgent(ctx, "support-bot", "v1")
//	defer agent.End()
//	// ... LLM calls and tool executions become child spans
package aiobs

import (
	"context"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"

	"github.com/yuhuibin/go-ai-obs/provider"
	"github.com/yuhuibin/go-ai-obs/recorder"
)

const Version = "0.2.0"

// Recorder is the main entry point. It manages tracing and metrics.
type Recorder struct {
	inner  *recorder.Recorder
	config Config
}

// New creates a new Recorder with functional options.
func New(opts ...Option) (*Recorder, error) {
	cfg := Config{
		SamplingRate: 1.0,
		ServiceName:  "unknown-service",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewWithConfig(cfg)
}

// NewWithConfig creates a new Recorder with a Config struct.
func NewWithConfig(cfg Config) (*Recorder, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "unknown-service"
	}
	if cfg.SamplingRate <= 0 {
		cfg.SamplingRate = 1.0
	}

	r, err := recorder.New(cfg.toRecorderConfig())
	if err != nil {
		return nil, fmt.Errorf("aiobs: %w", err)
	}

	return &Recorder{inner: r, config: cfg}, nil
}

// Shutdown gracefully stops the Recorder, flushing pending spans.
func (r *Recorder) Shutdown(ctx context.Context) error {
	return r.inner.Shutdown(ctx)
}

// MetricsHandler returns the HTTP handler for Prometheus /metrics.
func (r *Recorder) MetricsHandler() *recorder.Metrics {
	return r.inner.MetricsHandler()
}

// =============================================================================
// LLM Call Tracing
// =============================================================================

// TraceCall wraps an LLM call with GenAI-standard tracing and metrics.
//
// Example:
//
//	resp, err := aiobs.TraceCall(ctx, rec, provider.NewGemini(), req,
//	    func(ctx context.Context) (provider.GeminiResponse, error) {
//	        return myGeminiCall(ctx, req)
//	    })
func TraceCall[T any](ctx context.Context, rec *Recorder, p provider.AIProvider, req any, fn func(context.Context) (T, error)) (T, error) {
	ctx, finish := rec.inner.StartCall(ctx, p, req)
	resp, err := fn(ctx)
	finish(resp, err)
	return resp, err
}

// =============================================================================
// Agent & Tool Tracing
// =============================================================================

// StartAgent begins an agent invocation span. All LLM calls and tool executions
// in the returned context appear as children of this span.
//
// Example:
//
//	ctx, agent := rec.StartAgent(ctx, "support-bot", "v1.2",
//	    aiobs.AgentAttr("user_id", "user-123"),
//	)
//	defer agent.End()
func (r *Recorder) StartAgent(ctx context.Context, name, version string, attrs ...AgentAttr) (context.Context, *recorder.AgentSpan) {
	return r.inner.StartAgent(ctx, name, version)
}

// StartTool begins a tool execution span as a child of the current context.
//
// Example:
//
//	ctx, tool := rec.StartTool(ctx, "search_orders", "function")
//	result, err := searchOrders(ctx, query)
//	tool.End(result, err)
func (r *Recorder) StartTool(ctx context.Context, toolName, toolType string) (context.Context, *recorder.ToolSpan) {
	return r.inner.StartTool(ctx, toolName, toolType)
}

// StartChain creates a named child span for a pipeline step.
//
// Example:
//
//	ctx, end := rec.StartChain(ctx, "rag-retrieval")
//	docs, err := retrieveDocs(ctx, query)
//	end(err)
func (r *Recorder) StartChain(ctx context.Context, name string) (context.Context, func(err error)) {
	return r.inner.StartChain(ctx, name)
}

// AgentAttr is reserved for future agent attribute customization.
type AgentAttr struct {
	Key, Value string
}

// =============================================================================
// OpenAI Client Wrapper
// =============================================================================

// WrapOpenAI wraps an OpenAI client to automatically trace all calls.
func (r *Recorder) WrapOpenAI(client *openai.Client) *TracedOpenAIClient {
	return &TracedOpenAIClient{
		Client:   client,
		recorder: r,
		provider: provider.NewOpenAI(),
	}
}

// TracedOpenAIClient wraps *openai.Client, auto-instrumenting calls.
type TracedOpenAIClient struct {
	*openai.Client
	recorder *Recorder
	provider *provider.OpenAIProvider
}

// CreateChatCompletion traces a call to the OpenAI chat completion endpoint.
func (c *TracedOpenAIClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	ctx, finish := c.recorder.inner.StartCall(ctx, c.provider, req)
	resp, err := c.Client.CreateChatCompletion(ctx, req)
	finish(resp, err)
	return resp, err
}

// CreateChatCompletionStream traces a streaming OpenAI call, including TTFT and token throughput.
func (c *TracedOpenAIClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*TracedChatCompletionStream, error) {
	ctx, streamRec := c.recorder.inner.StartStreamCall(ctx, c.provider, req)

	stream, err := c.Client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		streamRec.Finish(nil, err)
		return nil, err
	}

	return &TracedChatCompletionStream{
		ChatCompletionStream: stream,
		streamRec:            streamRec,
		provider:             c.provider,
	}, nil
}

// TracedChatCompletionStream wraps an OpenAI stream to record TTFT and final metrics.
type TracedChatCompletionStream struct {
	*openai.ChatCompletionStream
	streamRec     *recorder.StreamRecorder
	provider      *provider.OpenAIProvider
	accumulated   openai.ChatCompletionResponse
	hasUsage      bool
	firstChunk    bool
	finished      bool
}

// Recv receives the next chunk. The first chunk triggers TTFT recording.
// On EOF, final metrics are recorded.
func (s *TracedChatCompletionStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	resp, err := s.ChatCompletionStream.Recv()

	if err == nil && !s.firstChunk {
		s.firstChunk = true
		s.streamRec.RecordFirstToken()
	}

	// Accumulate usage
	if resp.Usage != nil {
		s.accumulated.Usage = *resp.Usage
		s.hasUsage = true
	}
	if len(resp.Choices) > 0 && resp.Choices[0].FinishReason != "" {
		s.accumulated.Choices = []openai.ChatCompletionChoice{
			{FinishReason: resp.Choices[0].FinishReason},
		}
	}
	s.accumulated.Model = resp.Model
	s.streamRec.AddTokens(1) // approximate: 1 output token per chunk

	if err == io.EOF || err != nil {
		s.finish(err)
	}

	return resp, err
}

// Close closes the stream and records final metrics.
func (s *TracedChatCompletionStream) Close() error {
	err := s.ChatCompletionStream.Close()
	s.finish(nil)
	return err
}

func (s *TracedChatCompletionStream) finish(err error) {
	if s.finished {
		return
	}
	s.finished = true

	resp := s.accumulated
	if s.hasUsage {
		s.streamRec.Finish(resp, err)
	} else {
		s.streamRec.Finish(nil, err)
	}
}
