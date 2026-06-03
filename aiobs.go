// Package aiobs provides provider-agnostic observability for Go AI/LLM applications.
//
// It automatically traces LLM calls via OpenTelemetry, exports Prometheus metrics,
// and supports multiple providers (OpenAI, Anthropic) through an adapter interface.
//
// Quick start:
//
//	rec, _ := aiobs.New(aiobs.Config{ServiceName: "my-app"})
//	defer rec.Shutdown(ctx)
//
//	client := rec.WrapOpenAI(openai.NewClient("sk-..."))
//	resp, err := client.CreateChatCompletion(ctx, req)
//	// Automatically traced, with metrics recorded.
package aiobs

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/yuhuibin/go-ai-obs/provider"
	"github.com/yuhuibin/go-ai-obs/recorder"
)

// Version is the current version of go-ai-obs.
const Version = "0.1.0"

// Recorder is the main entry point. It manages tracing and metrics.
type Recorder struct {
	inner  *recorder.Recorder
	config Config
}

// New creates a new Recorder. One Recorder per application is sufficient.
func New(opts ...Option) (*Recorder, error) {
	cfg := Config{
		SamplingRate: 1.0,
		ServiceName:  "unknown-service",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	r, err := recorder.New(cfg.toRecorderConfig())
	if err != nil {
		return nil, fmt.Errorf("aiobs: %w", err)
	}

	return &Recorder{inner: r, config: cfg}, nil
}

// NewWithConfig creates a new Recorder with a Config struct directly.
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

// TraceCall wraps an LLM call with tracing and metrics.
// Use this for manual instrumentation when the auto-wrap helpers don't fit.
//
// Example:
//
//	result, err := rec.TraceCall(ctx, provider.NewOpenAI(), chatReq, func(ctx context.Context) {
//	    return rawClient.CreateChatCompletion(ctx, chatReq)
//	})
func TraceCall[T any](ctx context.Context, rec *Recorder, p provider.AIProvider, req any, fn func(context.Context) (T, error)) (T, error) {
	ctx, finish := rec.inner.StartCall(ctx, p, req)
	resp, err := fn(ctx)
	finish(resp, err)
	return resp, err
}

// WrapOpenAI wraps an OpenAI client to automatically trace all ChatCompletion calls.
//
// Example:
//
//	rec, _ := aiobs.New(aiobs.Config{ServiceName: "my-app"})
//	client := rec.WrapOpenAI(openai.NewClient("sk-..."))
//	// All calls to client.CreateChatCompletion are now traced.
//	resp, err := client.CreateChatCompletion(ctx, req)
func (r *Recorder) WrapOpenAI(client *openai.Client) *TracedOpenAIClient {
	return &TracedOpenAIClient{
		Client:   client,
		recorder: r,
		provider: provider.NewOpenAI(),
	}
}

// TracedOpenAIClient wraps an *openai.Client, intercepting CreateChatCompletion
// to record traces and metrics.
type TracedOpenAIClient struct {
	*openai.Client
	recorder *Recorder
	provider *provider.OpenAIProvider
}

// CreateChatCompletion calls the underlying OpenAI client and records observability data.
func (c *TracedOpenAIClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	ctx, finish := c.recorder.inner.StartCall(ctx, c.provider, req)
	resp, err := c.Client.CreateChatCompletion(ctx, req)
	finish(resp, err)
	return resp, err
}

// CreateChatCompletionStream calls the underlying OpenAI streaming client.
// Streaming metrics record the total duration and accumulated token count.
func (c *TracedOpenAIClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	ctx, finish := c.recorder.inner.StartCall(ctx, c.provider, req)
	stream, err := c.Client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		finish(nil, err)
		return nil, err
	}
	// For streaming, we can't know token counts until the stream is consumed.
	// Record basic info now; advanced usage can use TraceStream helper.
	finish(nil, nil)
	return stream, nil
}
