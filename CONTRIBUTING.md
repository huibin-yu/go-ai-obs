# Contributing to go-ai-obs

Thank you for your interest in contributing!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-username>/go-ai-obs.git`
3. Create a branch: `git checkout -b feat/my-feature`

## Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Run vet and static analysis
go vet ./...

# Run the example (requires running Jaeger locally)
docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest
go run _examples/basic/main.go
```

## Conventions

- Follow standard Go idioms (effective Go, Go Code Review Comments)
- Run `go fmt` and `go vet` before committing
- Write tests for new features
- Update the README if adding user-facing features
- Keep provider adapters in `provider/`
- Keep HTTP middleware in `middleware/`

## Pull Requests

1. Ensure your PR description clearly describes the problem and solution
2. Reference any related issues
3. Keep changes focused — one feature or fix per PR
4. All CI checks must pass before merge

## Adding a New Provider

Implement the `provider.AIProvider` interface:

```go
type AIProvider interface {
    Name() string
    ExtractRequest(req any) []attribute.KeyValue
    ExtractResponse(resp any, err error) CallInfo
    Cost(model string, inputTokens, outputTokens int) float64
}
```

Then register it via the `TraceCall` helper or submit a PR to add a built-in `WrapXxx` method.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
