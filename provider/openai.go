package provider

import (
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"go.opentelemetry.io/otel/attribute"

	"github.com/yuhuibin/go-ai-obs/semconv"
)

// OpenAI pricing per 1M tokens (as of 2026-06).
// https://openai.com/api/pricing/
var openAIPricing = map[string][2]float64{
	"gpt-4o":          {2.50, 10.00},
	"gpt-4o-mini":     {0.15, 0.60},
	"gpt-4.5-preview": {75.00, 150.00},
	"gpt-4-turbo":     {10.00, 30.00},
	"gpt-4":           {30.00, 60.00},
	"gpt-3.5-turbo":   {0.50, 1.50},
	"o3":              {10.00, 40.00},
	"o4-mini":         {1.10, 4.40},
	"gpt-4.1":         {2.00, 8.00},
	"gpt-4.1-mini":    {0.40, 1.60},
	"gpt-4.1-nano":    {0.10, 0.40},
}

// OpenAIProvider extracts observability data from OpenAI API calls.
type OpenAIProvider struct{}

// NewOpenAI returns a new OpenAIProvider.
func NewOpenAI() *OpenAIProvider {
	return &OpenAIProvider{}
}

func (p *OpenAIProvider) Name() string     { return "openai" }
func (p *OpenAIProvider) Operation() Operation { return OpChat }

// ExtractRequest extracts GenAI standard attributes from an OpenAI request.
func (p *OpenAIProvider) ExtractRequest(req any) []attribute.KeyValue {
	r, ok := req.(openai.ChatCompletionRequest)
	if !ok {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.String(semconv.AttrRequestModel, r.Model),
		attribute.Float64(semconv.AttrRequestTemperature, float64(r.Temperature)),
		attribute.Int(semconv.AttrRequestMaxTokens, r.MaxTokens),
		attribute.Int(semconv.AttrMessagesCount, len(r.Messages)),
	}

	if r.TopP > 0 {
		attrs = append(attrs, attribute.Float64(semconv.AttrRequestTopP, float64(r.TopP)))
	}
	if len(r.Stop) > 0 {
		attrs = append(attrs, attribute.StringSlice(semconv.AttrRequestStopSequences, r.Stop))
	}
	if r.Seed != nil {
		attrs = append(attrs, attribute.Int64(semconv.AttrRequestSeed, int64(*r.Seed)))
	}

	return attrs
}

// ExtractResponse extracts GenAI standard token usage and finish reason.
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
	info.ResponseID = r.ID
	info.InputTokens = r.Usage.PromptTokens
	info.OutputTokens = r.Usage.CompletionTokens

	if len(r.Choices) > 0 {
		info.FinishReason = string(r.Choices[0].FinishReason)
	}

	return info
}

// ExtractMessages extracts messages for opt-in content capture.
func (p *OpenAIProvider) ExtractMessages(req any, resp any) (input []Message, output []Message) {
	r, ok := req.(openai.ChatCompletionRequest)
	if ok {
		input = make([]Message, len(r.Messages))
		for i, m := range r.Messages {
			input[i] = Message{Role: m.Role, Content: m.Content}
		}
	}

	rr, ok := resp.(openai.ChatCompletionResponse)
	if ok {
		output = make([]Message, len(rr.Choices))
		for i, c := range rr.Choices {
			output[i] = Message{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			}
		}
	}

	return
}

// Cost returns the estimated cost for an OpenAI model call.
func (p *OpenAIProvider) Cost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := openAIPricing[model]
	if !ok {
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
