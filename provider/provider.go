// Package provider defines the interface for LLM provider adapters.
// Each provider (OpenAI, Anthropic, etc.) implements AIProvider to
// extract call-specific information for observability.
package provider

import "go.opentelemetry.io/otel/attribute"

// CallInfo holds the extracted information from an LLM call.
type CallInfo struct {
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	FinishReason string
}

// AIProvider extracts provider-specific information from LLM requests and responses.
// Implementations are stateless and safe for concurrent use.
type AIProvider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic").
	Name() string

	// ExtractRequest returns span attributes describing the request.
	ExtractRequest(req any) []attribute.KeyValue

	// ExtractResponse returns span attributes and token usage from the response.
	// The err parameter is the error returned by the LLM call, if any.
	ExtractResponse(resp any, err error) CallInfo

	// Cost returns the estimated cost in dollars for the given token usage.
	Cost(model string, inputTokens, outputTokens int) float64
}
