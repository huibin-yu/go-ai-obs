package provider

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	"github.com/yuhuibin/go-ai-obs/semconv"
)

// Anthropic pricing per 1M tokens (as of 2026-06).
var anthropicPricing = map[string][2]float64{
	"claude-opus-4-8":   {15.00, 75.00},
	"claude-sonnet-4-6": {3.00, 15.00},
	"claude-haiku-4-5":  {0.80, 4.00},
	"claude-opus-4":     {15.00, 75.00},
	"claude-sonnet-4":   {3.00, 15.00},
	"claude-haiku-3.5":  {0.80, 4.00},
	"claude-opus-3.5":   {15.00, 75.00},
	"claude-sonnet-3.5": {3.00, 15.00},
	"claude-haiku-3":    {0.25, 1.25},
}

// AnthropicProvider extracts observability data from Anthropic API calls.
type AnthropicProvider struct{}

// NewAnthropic returns a new AnthropicProvider.
func NewAnthropic() *AnthropicProvider {
	return &AnthropicProvider{}
}

func (p *AnthropicProvider) Name() string        { return "anthropic" }
func (p *AnthropicProvider) Operation() Operation { return OpChat }

// AnthropicMessage represents a message in the Anthropic API.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicRequest captures Anthropic request fields for tracing.
type AnthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   float64            `json:"temperature"`
	TopP          float64            `json:"top_p"`
	TopK          int                `json:"top_k,omitempty"`
	Messages      []AnthropicMessage `json:"messages"`
	System        string             `json:"system,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
}

// AnthropicResponse captures Anthropic response fields for tracing.
type AnthropicResponse struct {
	ID         string         `json:"id"`
	Model      string         `json:"model"`
	Usage      AnthropicUsage `json:"usage"`
	StopReason string         `json:"stop_reason"`
	Content    []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// AnthropicUsage holds token usage from Anthropic responses.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ExtractRequest extracts GenAI standard attributes from an Anthropic request.
func (p *AnthropicProvider) ExtractRequest(req any) []attribute.KeyValue {
	r, ok := req.(AnthropicRequest)
	if !ok {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.String(semconv.AttrRequestModel, r.Model),
		attribute.Float64(semconv.AttrRequestTemperature, r.Temperature),
		attribute.Int(semconv.AttrRequestMaxTokens, r.MaxTokens),
		attribute.Int(semconv.AttrMessagesCount, len(r.Messages)),
	}

	if r.System != "" {
		attrs = append(attrs, attribute.Bool(semconv.AttrHasSystemPrompt, true))
	}
	if r.TopP > 0 {
		attrs = append(attrs, attribute.Float64(semconv.AttrRequestTopP, r.TopP))
	}
	if r.TopK > 0 {
		attrs = append(attrs, attribute.Int(semconv.AttrRequestTopK, r.TopK))
	}
	if len(r.StopSequences) > 0 {
		attrs = append(attrs, attribute.StringSlice(semconv.AttrRequestStopSequences, r.StopSequences))
	}

	return attrs
}

// ExtractResponse extracts GenAI standard token usage and stop reason.
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
	info.ResponseID = r.ID
	info.InputTokens = r.Usage.InputTokens
	info.OutputTokens = r.Usage.OutputTokens
	info.FinishReason = r.StopReason

	return info
}

// ExtractMessages extracts messages for opt-in content capture.
func (p *AnthropicProvider) ExtractMessages(req any, resp any) (input []Message, output []Message) {
	r, ok := req.(AnthropicRequest)
	if ok {
		input = make([]Message, len(r.Messages))
		for i, m := range r.Messages {
			input[i] = Message{Role: m.Role, Content: m.Content}
		}
		if r.System != "" {
			input = append([]Message{{Role: "system", Content: r.System}}, input...)
		}
	}

	rr, ok := resp.(AnthropicResponse)
	if ok {
		output = make([]Message, len(rr.Content))
		for i, c := range rr.Content {
			output[i] = Message{Role: "assistant", Content: c.Text}
		}
	}

	return
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
