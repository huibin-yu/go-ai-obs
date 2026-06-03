package provider

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

// Anthropic pricing per 1M tokens (as of 2026-06).
var anthropicPricing = map[string][2]float64{
	"claude-opus-4-8":      {15.00, 75.00},
	"claude-sonnet-4-6":    {3.00, 15.00},
	"claude-haiku-4-5":     {0.80, 4.00},
	"claude-opus-4":        {15.00, 75.00},
	"claude-sonnet-4":      {3.00, 15.00},
	"claude-haiku-3.5":     {0.80, 4.00},
	"claude-opus-3.5":      {15.00, 75.00},
	"claude-sonnet-3.5":    {3.00, 15.00},
	"claude-haiku-3":       {0.25, 1.25},
}

// AnthropicProvider extracts observability data from Anthropic API calls.
type AnthropicProvider struct{}

// NewAnthropic returns a new AnthropicProvider.
func NewAnthropic() *AnthropicProvider {
	return &AnthropicProvider{}
}

// Name returns "anthropic".
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// AnthropicMessage represents an Anthropic message for type assertion.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicRequest is a generic struct capturing Anthropic request fields.
type AnthropicRequest struct {
	Model       string              `json:"model"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature"`
	TopP        float64             `json:"top_p"`
	Messages    []AnthropicMessage  `json:"messages"`
	System      string              `json:"system,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
}

// AnthropicResponse is a generic struct capturing Anthropic response fields.
type AnthropicResponse struct {
	Model  string              `json:"model"`
	Usage  AnthropicUsage      `json:"usage"`
	StopReason string          `json:"stop_reason"`
}

// AnthropicUsage holds token usage from Anthropic responses.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ExtractRequest extracts attributes from an Anthropic messages request.
func (p *AnthropicProvider) ExtractRequest(req any) []attribute.KeyValue {
	r, ok := req.(AnthropicRequest)
	if !ok {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.String("llm.model", r.Model),
		attribute.Float64("llm.temperature", r.Temperature),
		attribute.Int("llm.max_tokens", r.MaxTokens),
		attribute.Int("llm.messages_count", len(r.Messages)),
	}

	if r.System != "" {
		attrs = append(attrs, attribute.Bool("llm.has_system_prompt", true))
	}

	if r.TopP > 0 {
		attrs = append(attrs, attribute.Float64("llm.top_p", r.TopP))
	}

	return attrs
}

// ExtractResponse extracts token usage and stop reason from an Anthropic response.
func (p *AnthropicProvider) ExtractResponse(resp any, err error) CallInfo {
	info := CallInfo{Provider: "anthropic"}

	if err != nil {
		info.FinishReason = "error"
		return info
	}

	r, ok := resp.(AnthropicResponse)
	if !ok {
		return info
	}

	info.Model = r.Model
	info.InputTokens = r.Usage.InputTokens
	info.OutputTokens = r.Usage.OutputTokens
	info.FinishReason = r.StopReason

	return info
}

// Cost returns the estimated cost for an Anthropic model call.
func (p *AnthropicProvider) Cost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := anthropicPricing[model]
	if !ok {
		return 0
	}

	inputCost := (float64(inputTokens) / 1_000_000) * pricing[0]
	outputCost := (float64(outputTokens) / 1_000_000) * pricing[1]

	return inputCost + outputCost
}

// SetAnthropicPricing adds or overrides pricing for an Anthropic model.
func SetAnthropicPricing(model string, inputPer1M, outputPer1M float64) {
	anthropicPricing[model] = [2]float64{inputPer1M, outputPer1M}
}

// FormatAnthropicCost returns a human-readable cost string.
func FormatAnthropicCost(dollars float64) string {
	if dollars < 0.01 {
		return fmt.Sprintf("$%.4f", dollars)
	}
	return fmt.Sprintf("$%.2f", dollars)
}
