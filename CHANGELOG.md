# Changelog

## [0.2.0] - 2026-06-03

### Added

- **GenAI Semantic Conventions**: all span attributes migrated to `gen_ai.*` standard (OpenTelemetry GenAI spec), compatible with Langfuse, Arize Phoenix, Datadog, and any OTLP backend
- **Agent & Tool Tracing**: `StartAgent()`, `StartTool()`, `StartChain()` for parent-child span hierarchy in multi-step AI workflows
- **Streaming TTFT**: time-to-first-token histogram (`aiobs_llm_ttft_seconds`) and tokens-per-second metric
- **Google AI (Gemini) Provider**: full adapter with `GeminiRequest`/`GeminiResponse` types, pricing, and `ExtractMessages` support
- **Message Content Capture**: `WithCaptureContent()` opt-in to record `gen_ai.input.messages` and `gen_ai.output.messages`
- **gRPC Middleware**: `UnaryServerInterceptor` and `StreamServerInterceptor` with trace context propagation
- **Grafana Dashboard**: ready-to-use dashboard JSON in `dashboards/`
- **StreamRecorder**: new `StartStreamCall` API that records TTFT and tokens-per-second
- `gen_ai.response.id` attribute for support ticket correlation
- 50+ unit tests across all packages

### Changed

- **Breaking**: Span attribute keys changed from `llm.*` to `gen_ai.*` per OpenTelemetry GenAI standard
- **Breaking**: `AIProvider` interface now requires `Operation()` and `ExtractMessages()` methods
- Span names now follow `"chat <model>"` convention (e.g., `chat gpt-4o`)
- `Metrics` now supports custom Prometheus registries via `NewMetricsWithRegistry`
- `Recorder.Config` gains `CaptureContent` and `MetricsRegistry` fields
- Version bumped to 0.2.0

## [0.1.0] - 2026-06-03

### Added

- Initial release
- Core `Recorder` with OpenTelemetry span lifecycle management
- OpenAI provider adapter with automatic tracing for `CreateChatCompletion`
- Anthropic provider adapter with generic request/response types
- Prometheus metrics: `aiobs_llm_requests_total`, `aiobs_llm_tokens_total`, `aiobs_llm_latency_seconds`, `aiobs_llm_cost_dollars_total`
- OTLP gRPC trace exporter
- `WrapOpenAI` convenience method for one-line instrumentation
- `TraceCall[T]` generic helper for manual/custom provider instrumentation
- Gin middleware with trace context propagation
- Generic HTTP handler wrapper
- Up-to-date OpenAI and Anthropic model pricing
- Basic example with Jaeger integration

[0.2.0]: https://github.com/huibin-yu/go-ai-obs/releases/tag/v0.2.0
[0.1.0]: https://github.com/huibin-yu/go-ai-obs/releases/tag/v0.1.0
