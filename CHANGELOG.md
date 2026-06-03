# Changelog

All notable changes to go-ai-obs will be documented in this file.

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

[0.1.0]: https://github.com/yuhuibin/go-ai-obs/releases/tag/v0.1.0
