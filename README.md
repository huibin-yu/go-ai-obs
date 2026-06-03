# go-ai-obs

[![Go Reference](https://pkg.go.dev/badge/github.com/yuhuibin/go-ai-obs.svg)](https://pkg.go.dev/github.com/yuhuibin/go-ai-obs)
[![Go Report Card](https://goreportcard.com/badge/github.com/yuhuibin/go-ai-obs)](https://goreportcard.com/report/github.com/yuhuibin/go-ai-obs)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Go-native, provider-agnostic observability for AI/LLM applications.**

`go-ai-obs` auto-instruments your LLM calls with OpenTelemetry traces (GenAI semantic conventions), Prometheus metrics, and structured logging — monitor latency, token usage, cost, and TTFT across OpenAI, Google AI, and other providers.

## ✨ Features

- 🎯 **GenAI Semantic Conventions** — `gen_ai.*` attributes per OpenTelemetry spec, compatible with Langfuse, Arize Phoenix, Datadog, and any OTLP backend
- 🤖 **Agent & Tool Tracing** — parent-child span hierarchy for multi-step agent workflows with automatic tool call instrumentation
- ⚡ **Streaming TTFT** — time-to-first-token and tokens-per-second for streaming LLM calls
- 🔌 **Multi-Provider** — OpenAI and Google AI (Gemini) adapters; add your own via `AIProvider` interface
- 📊 **OpenTelemetry Traces** — OTLP gRPC export to Jaeger, Tempo, Grafana
- 📈 **Prometheus Metrics** — requests, tokens, latency, cost, TTFT out of the box
- 🧩 **Framework Middleware** — Gin and gRPC interceptors with trace propagation
- 💰 **Cost Tracking** — automatic per-call cost estimation with up-to-date pricing for all providers
- 🔒 **PII-Safe by Default** — message content capture is opt-in via `WithCaptureContent()`
- 📋 **Grafana Dashboard** — ready-to-use dashboard JSON in `dashboards/`

## 📦 Installation

```bash
go get github.com/yuhuibin/go-ai-obs
```

Requires Go 1.25+.

## 🚀 Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"

    openai "github.com/sashabaranov/go-openai"
    aiobs "github.com/yuhuibin/go-ai-obs"
)

func main() {
    rec, err := aiobs.New(
        aiobs.WithServiceName("my-ai-app"),
        aiobs.WithEnvironment("production"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer rec.Shutdown(context.Background())

    http.Handle("/metrics", rec.MetricsHandler().Handler())
    go http.ListenAndServe(":9090", nil)

    client := rec.WrapOpenAI(openai.NewClient("sk-..."))

    resp, _ := client.CreateChatCompletion(context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4o,
            Messages: []openai.ChatCompletionMessage{
                {Role: "user", Content: "Hello!"},
            },
        },
    )
    log.Printf("Response: %s", resp.Choices[0].Message.Content)
}
```

Run with Jaeger:

```bash
docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest
go run main.go
```

Traces at http://localhost:16686, metrics at http://localhost:9090/metrics.

## 🤖 Agent & Tool Tracing

```go
// Create an agent span — all LLM calls within become children
ctx, agent := rec.StartAgent(ctx, "support-bot", "v1.2")
defer agent.End()

// LLM call becomes a child of the agent span
resp, _ := client.CreateChatCompletion(ctx, req)

// Tool execution also becomes a child
ctx, tool := rec.StartTool(ctx, "search_orders", "function")
result, _ := searchOrders(ctx, "order-123")
tool.End(result, nil)

// Generic chain step
ctx, end := rec.StartChain(ctx, "rag-retrieval")
docs, _ := retrieveDocs(ctx, query)
end(nil)
```

Span hierarchy in Jaeger:
```
invoke_agent support-bot
├── chat gpt-4o
├── execute_tool search_orders
└── rag-retrieval
```

## ⚡ Streaming with TTFT

```go
stream, _ := client.CreateChatCompletionStream(ctx, req)
for {
    chunk, err := stream.Recv()
    if err == io.EOF { break }
    fmt.Print(chunk.Choices[0].Delta.Content)
}
stream.Close()
// TTFT, tokens/s, and total usage automatically recorded
```

## 📊 Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `aiobs_llm_requests_total` | Counter | LLM requests by service, model, provider, status |
| `aiobs_llm_tokens_total` | Counter | Token consumption (input/output) by model |
| `aiobs_llm_latency_seconds` | Histogram | End-to-end call duration |
| `aiobs_llm_cost_dollars_total` | Counter | Estimated cumulative cost |
| `aiobs_llm_ttft_seconds` | Histogram | Time-to-first-token (streaming) |

## 🔌 Providers

### OpenAI (built-in)

```go
client := rec.WrapOpenAI(openai.NewClient("sk-..."))
// CreateChatCompletion and CreateChatCompletionStream auto-traced
```

### Google AI / Gemini (built-in)

```go
resp, err := aiobs.TraceCall(ctx, rec, provider.NewGemini(), req,
    func(ctx context.Context) (provider.GeminiResponse, error) {
        return geminiResp, nil
    })
```

### Custom Provider

```go
type MyProvider struct{}

func (p *MyProvider) Name() string             { return "custom" }
func (p *MyProvider) Operation() provider.Operation { return provider.OpChat }
func (p *MyProvider) ExtractRequest(req any) []attribute.KeyValue { ... }
func (p *MyProvider) ExtractResponse(resp any, err error) provider.CallInfo { ... }
func (p *MyProvider) ExtractMessages(req, resp any) ([]provider.Message, []provider.Message) { ... }
func (p *MyProvider) Cost(model string, in, out int) float64 { ... }

aiobs.TraceCall(ctx, rec, &MyProvider{}, req, myFunc)
```

## 🧩 Middleware

```go
// Gin
r := gin.Default()
r.Use(middleware.GinMiddleware("my-service"))

// gRPC Unary
s := grpc.NewServer(
    grpc.UnaryInterceptor(middleware.UnaryServerInterceptor("my-service")),
)

// gRPC Stream
s := grpc.NewServer(
    grpc.StreamInterceptor(middleware.StreamServerInterceptor("my-service")),
)
```

## 📋 Grafana Dashboard

Import `dashboards/grafana-llm-observability.json` into Grafana for a ready-to-use LLM observability dashboard with request rates, token usage, latency percentiles, cost tracking, and TTFT.

## 🏗 Architecture

```
User Code
  │
  ▼
aiobs.WrapOpenAI / TraceCall / StartAgent / StartTool
  │
  ├── Trace  (OTLP gRPC → Jaeger/Tempo/Datadog/Honeycomb)
  ├── Metrics (Prometheus endpoint)
  └── Provider Adapter (OpenAI / Gemini / Custom)
```

All spans use OpenTelemetry GenAI Semantic Conventions (`gen_ai.*` attributes), making them natively compatible with any OTLP-compatible backend.

## 🧪 Development

```bash
git clone https://github.com/huibin-yu/go-ai-obs.git
cd go-ai-obs

go test ./...     # Run tests
go vet ./...      # Static analysis

# Run example
docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest
OPENAI_API_KEY=sk-... go run _examples/basic/main.go
```

## 📝 License

MIT — see [LICENSE](LICENSE).

## 🙋 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).
