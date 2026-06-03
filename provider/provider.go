// Package provider defines the interface for LLM provider adapters.
// Each provider (OpenAI, Google AI, etc.) implements AIProvider to
// extract call-specific information for observability.
package provider

import "go.opentelemetry.io/otel/attribute"

// Operation represents the type of GenAI operation.
type Operation string

const (
	OpChat           Operation = "chat"
	OpTextCompletion Operation = "text_completion"
	OpEmbeddings     Operation = "embeddings"
)

// CallInfo holds the extracted information from an LLM call.
type CallInfo struct {
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	FinishReason string
	ResponseID   string
}

// Message represents a chat message for content capture.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIProvider extracts provider-specific information from LLM requests and responses.
// Implementations are stateless and safe for concurrent use.
type AIProvider interface {
	// Name returns the provider identifier (e.g., "openai", "gemini").
	Name() string

	// Operation returns the GenAI operation type for this call.
	Operation() Operation

	// ExtractRequest returns span attributes describing the request.
	ExtractRequest(req any) []attribute.KeyValue

	// ExtractResponse returns span attributes and token usage from the response.
	// The err parameter is the error returned by the LLM call, if any.
	ExtractResponse(resp any, err error) CallInfo

	// ExtractMessages extracts chat messages from request and response for content capture.
	// Returns nil if content capture is disabled or unsupported.
	ExtractMessages(req any, resp any) (input []Message, output []Message)

	// Cost returns the estimated cost in dollars for the given token usage.
	Cost(model string, inputTokens, outputTokens int) float64
}
