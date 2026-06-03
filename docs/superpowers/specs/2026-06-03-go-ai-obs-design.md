# go-ai-obs Design Spec

## Overview

**go-ai-obs** is a Go-native, provider-agnostic observability library for AI/LLM applications. It provides automatic tracing, metrics, and logging for every LLM call, built on OpenTelemetry standards with zero-friction integration.

- **Target users**: Go developers building AI-powered applications who need production-grade observability
- **License**: MIT
- **Language**: Go 1.22+

## Goals & Non-Goals

### Goals
- Auto-instrument LLM calls with OpenTelemetry traces (Spans)
- Export Prometheus metrics (latency, token usage, cost)
- Support multiple LLM providers via adapter pattern (OpenAI first, Gemini second)
- Provide framework middleware (Gin first)
- Minimize user-code intrusion (ideally 1-3 lines to enable)

### Non-Goals
- Not an Agent framework (complements eino/langchaingo, not replaces them)
- Not a model router or gateway
- No built-in dashboard (use existing tools: Jaeger, Grafana, etc.)
- No Python/Node.js port (Go native)

## Architecture

Three-layer design, dependencies flow top-down:

```
User Code (Gin/Hertz/gRPC + Agent)
  |
  v
go-ai-obs.Wrap(client, opts...)
  |
  +-- Trace (OTLP gRPC -> Jaeger/Tempo)
  +-- Metrics (Prometheus endpoint)
  +-- Logger (slog structured -> Loki/ELK)
  |
  +-- Core Recorder (Span lifecycle)
       |
       +-- OpenAI Adapter
       +-- Gemini Adapter
       +-- Custom Adapter
```

### Key Design Decisions

1. **Wrapper pattern over middleware injection** — wrapping the client is less magical, easier to debug, and works with any downstream code. Middleware is provided for HTTP framework integration (request_id, user_id propagation).

2. **Adapter per provider** — each provider has different API shapes for request/response. Adapters extract provider-specific info (token counts, model names) into a unified `AICallRecord`.

3. **OpenTelemetry as the wire protocol** — traces use OTLP (gRPC), metrics use Prometheus exposition format. This is the industry standard and avoids vendor lock-in.

## API Design

### Minimal Setup
```go
client := aiobs.Wrap(openai.NewClient(apiKey),
    aiobs.WithServiceName("my-app"),
)
// All subsequent calls are automatically traced
resp, err := client.Chat.Completions.New(ctx, params)
```

### Options
```go
aiobs.WithServiceName("name")          // Service name in traces/metrics
aiobs.WithCustomAttr("key", "value")   // Static custom attributes
aiobs.WithSamplingRate(0.1)            // Trace sampling (0.0-1.0)
aiobs.WithCostConfig(CostConfig{...})  // Custom model pricing
aiobs.WithLogger(slog.Logger)          // Custom logger
aiobs.WithTracerProvider(tp)           // Custom OTel TracerProvider
```

### Prometheus Metrics
```
aiobs_llm_requests_total{service,model,provider,status}
aiobs_llm_tokens_total{service,model,provider,type}    // type: input|output
aiobs_llm_latency_seconds{service,model,provider}      // histogram
aiobs_llm_cost_dollars_total{service,model,provider}
```

### Middleware
```go
// Gin: auto-inject request_id into spans
r.Use(aiobs.GinMiddleware("service-name"))
```

### Manual Chain Span
```go
ctx, span := aiobs.TraceChain(ctx, "rag-retrieval")
defer span.End()
```

## Core Interface

```go
// AIProvider is implemented by each LLM provider adapter.
type AIProvider interface {
    // ExtractRequest reads model, messages, temperature from the call context.
    ExtractRequest(span trace.Span, req any)
    // ExtractResponse reads token usage, finish_reason from the response.
    ExtractResponse(span trace.Span, resp any)
    // Cost returns the dollar cost for given model and token counts.
    Cost(model string, inputTokens, outputTokens int) float64
}
```

The Core Recorder:
- Starts a Span before the LLM call
- Calls `provider.ExtractRequest()` to add request attributes
- Calls `provider.ExtractResponse()` after the call returns
- Records latency, token counts, cost as span attributes and metrics
- Ends the Span, setting status to error if applicable

## Project Structure

```
go-ai-obs/
├── .github/workflows/ci.yml
├── aiobs.go              // Public API: Wrap(), Version, TraceChain()
├── config.go             // Option type and constructors
├── recorder/
│   ├── recorder.go       // Core Recorder
│   ├── tracer.go         // OTLP TracerProvider setup
│   └── metrics.go        // Prometheus metrics registration + /metrics handler
├── provider/
│   ├── provider.go       // AIProvider interface
│   ├── openai.go         // OpenAI adapter (go-openai v2)
│   └── gemini.go         // Google AI adapter
├── middleware/
│   ├── gin.go            // Gin middleware
│   └── generic.go        // Generic http.Handler wrapper
├── _examples/
│   ├── basic/main.go     // Minimal working example
│   └── gin-demo/main.go  // Gin integration example
├── go.mod
├── README.md
├── LICENSE               // MIT
├── CHANGELOG.md
└── CONTRIBUTING.md
```

## Dependencies

Required:
- `go.opentelemetry.io/otel` — core OTel API
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace` — OTLP trace export
- `github.com/prometheus/client_golang` — Prometheus metrics
- `github.com/sashabaranov/go-openai` — OpenAI Go SDK (Phase 1)

Optional (later phases):
- Google AI SDK — Gemini adapter (Phase 3)
- `github.com/gin-gonic/gin` — Gin middleware (Phase 3)

## Implementation Phases

### Phase 1: Core Skeleton
- `go.mod` initialization
- `AIProvider` interface in `provider/`
- `OpenAIProvider` with request/response extraction and cost calculation
- `recorder/recorder.go` — basic Span lifecycle with OpenAI calls
- `config.go` — Option type, functional options pattern
- `aiobs.go` — `Wrap()` function for OpenAI client

### Phase 2: Trace & Metrics Complete
- `recorder/tracer.go` — OTLP gRPC exporter setup
- `recorder/metrics.go` — Prometheus metrics and `/metrics` endpoint
- `_examples/basic/` — working demo with local Jaeger

### Phase 3: Multi-Provider & Middleware
- `provider/gemini.go` — Google AI adapter
- `middleware/gin.go` — Gin middleware
- `middleware/generic.go` — Generic HTTP wrapper

### Phase 4: Polish & Release
- README with badges, quickstart, examples
- CHANGELOG.md
- CONTRIBUTING.md
- CI workflow (lint, test, build)
- Tag v0.1.0

## Codex Application Strategy

This project is designed to qualify for Codex for OSS:
- **Unique value**: Go ecosystem lacks a dedicated AI observability library. Existing solutions are Python-only or generic OTel.
- **Ecosystem importance**: As more Go services integrate LLMs (evidenced by projects like eino, langchaingo, mcp-go), observability becomes critical infrastructure.
- **AI usage narrative for application**: Codex will be used for PR review automation, test generation, documentation generation, and release note drafting — all natural workflows for a library maintainer.
- **Activity plan**: Regular releases, responsive issue management, community engagement through the CNCF/OTel community.
