package provider

import (
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"go.opentelemetry.io/otel/attribute"
)

// OpenAI pricing per 1M tokens (as of 2026-06).
// https://openai.com/api/pricing/
var openAIPricing = map[string][2]float64{
	// {input, output} per 1M tokens
	"gpt-4o":                  {2.50, 10.00},
	"gpt-4o-mini":             {0.15, 0.60},
	"gpt-4.5-preview":         {75.00, 150.00},
	"gpt-4-turbo":             {10.00, 30.00},
	"gpt-4":                   {30.00, 60.00},
	"gpt-3.5-turbo":           {0.50, 1.50},
	"o3":                      {10.00, 40.00},
	"o4-mini":                 {1.10, 4.40},
	"gpt-4.1":                 {2.00, 8.00},
	"gpt-4.1-mini":            {0.40, 1.60},
	"gpt-4.1-nano":            {0.10, 0.40},
}

// OpenAIProvider extracts observability data from OpenAI API calls.
type OpenAIProvider struct{}

// NewOpenAI returns a new OpenAIProvider.
func NewOpenAI() *OpenAIProvider {
	return &OpenAIProvider{}
}

// Name returns "openai".
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// ExtractRequest extracts attributes from an OpenAI chat completion request.
func (p *OpenAIProvider) ExtractRequest(req any) []attribute.KeyValue {
	r, ok := req.(openai.ChatCompletionRequest)
	if !ok {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.String("llm.model", r.Model),
		attribute.Float64("llm.temperature", float64(r.Temperature)),
		attribute.Int("llm.max_tokens", r.MaxTokens),
		attribute.Int("llm.messages_count", len(r.Messages)),
	}

	if r.TopP > 0 {
		attrs = append(attrs, attribute.Float64("llm.top_p", float64(r.TopP)))
	}

	if len(r.Stop) > 0 {
		attrs = append(attrs, attribute.StringSlice("llm.stop", r.Stop))
	}

	return attrs
}

// ExtractResponse extracts token usage and finish reason from an OpenAI response.
func (p *OpenAIProvider) ExtractResponse(resp any, err error) CallInfo {
	info := CallInfo{Provider: "openai"}

	if err != nil {
		info.FinishReason = "error"
		return info
	}

	r, ok := resp.(openai.ChatCompletionResponse)
	if !ok {
		return info
	}

	info.Model = r.Model
	info.InputTokens = r.Usage.PromptTokens
	info.OutputTokens = r.Usage.CompletionTokens

	if len(r.Choices) > 0 {
		info.FinishReason = string(r.Choices[0].FinishReason)
	}

	return info
}

// Cost returns the estimated cost for an OpenAI model call.
// Returns 0 if the model pricing is unknown.
func (p *OpenAIProvider) Cost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := openAIPricing[model]
	if !ok {
		// Unknown model — return 0 rather than guessing
		return 0
	}

	inputCost := (float64(inputTokens) / 1_000_000) * pricing[0]
	outputCost := (float64(outputTokens) / 1_000_000) * pricing[1]

	return inputCost + outputCost
}

// RegisterPricing adds or overrides pricing for a model.
func RegisterPricing(model string, inputPer1M, outputPer1M float64) {
	openAIPricing[model] = [2]float64{inputPer1M, outputPer1M}
}

// FormatCost returns a human-readable cost string.
func FormatCost(dollars float64) string {
	if dollars < 0.01 {
		return fmt.Sprintf("$%.4f", dollars)
	}
	return fmt.Sprintf("$%.2f", dollars)
}
