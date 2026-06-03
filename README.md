# go-ai-obs

[![Go Reference](https://pkg.go.dev/badge/github.com/yuhuibin/go-ai-obs.svg)](https://pkg.go.dev/github.com/yuhuibin/go-ai-obs)
[![Go Report Card](https://goreportcard.com/badge/github.com/yuhuibin/go-ai-obs)](https://goreportcard.com/report/github.com/yuhuibin/go-ai-obs)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Go-native, provider-agnostic observability for AI/LLM applications.**

`go-ai-obs` auto-instruments your LLM calls with OpenTelemetry traces, Prometheus metrics, and structured logging — so you can monitor latency, token usage, and cost across any provider.

## ✨ Features

- 🎯 **Zero-friction setup** — wrap your existing client in one line
- 🔌 **Provider-agnostic** — OpenAI and Anthropic adapters included; add your own
- 📊 **OpenTelemetry Traces** — OTLP gRPC export to Jaeger, Tempo, Grafana
- 📈 **Prometheus Metrics** — requests, tokens, latency, cost out of the box
- 🧩 **Framework Middleware** — Gin support built-in; trace propagation across HTTP
- 💰 **Cost Tracking** — automatic per-call cost estimation with up-to-date pricing

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
    _ "github.com/yuhuibin/go-ai-obs/middleware"
)

func main() {
    // 1. Create the recorder
    rec, err := aiobs.New(
        aiobs.WithServiceName("my-ai-app"),
        aiobs.WithEnvironment("production"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer rec.Shutdown(context.Background())

    // 2. Expose Prometheus metrics
    http.Handle("/metrics", rec.MetricsHandler().Handler())
    go http.ListenAndServe(":9090", nil)

    // 3. Wrap your OpenAI client
    rawClient := openai.NewClient("sk-...")
    client := rec.WrapOpenAI(rawClient)

    // 4. All calls are now automatically traced
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

Run with a local Jaeger instance:

```bash
docker run -d --name jaeger \
  -p 16686:16686 -p 4317:4317 \
  jaegertracing/all-in-one:latest

go run main.go
```

Open http://localhost:16686 to view traces, and http://localhost:9090/metrics for Prometheus metrics.

## 📊 Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `aiobs_llm_requests_total` | Counter | service, model, provider, status | Total LLM requests |
| `aiobs_llm_tokens_total` | Counter | service, model, provider, type | Tokens consumed (input/output) |
| `aiobs_llm_latency_seconds` | Histogram | service, model, provider | Call duration |
| `aiobs_llm_cost_dollars_total` | Counter | service, model, provider | Estimated cost |

## 🧩 Architecture

```
User Code
  │
  ▼
aiobs.WrapOpenAI(client)  ◄── One-line instrumentation
  │
  ├── Trace  (OTLP gRPC → Jaeger/Tempo/Grafana)
  ├── Metrics (Prometheus endpoint)
  └── Provider Adapter (OpenAI / Anthropic / Custom)
```

## 🔌 Providers

### OpenAI (built-in)

```go
client := rec.WrapOpenAI(openai.NewClient("sk-..."))
// CreateChatCompletion and CreateChatCompletionStream are auto-traced
```

### Anthropic (built-in)

```go
resp, err := aiobs.TraceCall(ctx, rec, provider.NewAnthropic(), req,
    func(ctx context.Context) (provider.AnthropicResponse, error) {
        // Your Anthropic SDK call here
        return anthropicResp, nil
    })
```

### Custom Provider

```go
type MyProvider struct{}

func (p *MyProvider) Name() string { return "my-llm" }
func (p *MyProvider) ExtractRequest(req any) []attribute.KeyValue { ... }
func (p *MyProvider) ExtractResponse(resp any, err error) provider.CallInfo { ... }
func (p *MyProvider) Cost(model string, in, out int) float64 { ... }

// Use it
aiobs.TraceCall(ctx, rec, &MyProvider{}, req, myLLMFunc)
```

## 🧪 Local Development

```bash
# Clone
git clone https://github.com/yuhuibin/go-ai-obs.git
cd go-ai-obs

# Run tests
go test ./...

# Run example (requires Jaeger)
docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest
OPENAI_API_KEY=sk-... go run _examples/basic/main.go
```

## 📝 License

MIT — see [LICENSE](LICENSE) for details.

## 🙋 Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
