package provider

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	"github.com/yuhuibin/go-ai-obs/semconv"
)

// Gemini pricing per 1M tokens (as of 2026-06).
// https://ai.google.dev/pricing
var geminiPricing = map[string][2]float64{
	"gemini-2.5-pro":     {1.25, 10.00},
	"gemini-2.5-flash":   {0.15, 0.60},
	"gemini-2.0-flash":   {0.10, 0.40},
	"gemini-2.0-flash-lite": {0.075, 0.30},
	"gemini-1.5-pro":     {1.25, 5.00},
	"gemini-1.5-flash":   {0.075, 0.30},
}

// GeminiProvider extracts observability data from Google AI (Gemini) API calls.
// Uses generic request/response structs for maximum compatibility.
type GeminiProvider struct{}

// NewGemini returns a new GeminiProvider.
func NewGemini() *GeminiProvider {
	return &GeminiProvider{}
}

func (p *GeminiProvider) Name() string        { return "gemini" }
func (p *GeminiProvider) Operation() Operation { return OpChat }

// GeminiContent represents a content block in Gemini API.
type GeminiContent struct {
	Role  string           `json:"role"`
	Parts []GeminiPart     `json:"parts"`
}

// GeminiPart is a part of a Gemini content block.
type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

// GeminiRequest captures Gemini request fields for tracing.
type GeminiRequest struct {
	Model       string          `json:"model"`
	Contents    []GeminiContent `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generation_config,omitempty"`
}

// GeminiGenerationConfig is the generation config for Gemini.
type GeminiGenerationConfig struct {
	MaxOutputTokens int     `json:"max_output_tokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"top_p,omitempty"`
	TopK            int     `json:"top_k,omitempty"`
	StopSequences   []string `json:"stop_sequences,omitempty"`
}

// GeminiResponse captures Gemini response fields for tracing.
type GeminiResponse struct {
	Model   string           `json:"model"`
	Candidates []GeminiCandidate `json:"candidates"`
	UsageMetadata *GeminiUsageMetadata `json:"usage_metadata,omitempty"`
}

// GeminiCandidate is a response candidate.
type GeminiCandidate struct {
	FinishReason string         `json:"finish_reason"`
	Content      GeminiContent  `json:"content"`
}

// GeminiUsageMetadata holds token usage from Gemini responses.
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"prompt_token_count"`
	CandidatesTokenCount int `json:"candidates_token_count"`
	TotalTokenCount      int `json:"total_token_count"`
}

// ExtractRequest extracts GenAI standard attributes from a Gemini request.
func (p *GeminiProvider) ExtractRequest(req any) []attribute.KeyValue {
	r, ok := req.(GeminiRequest)
	if !ok {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.String(semconv.AttrRequestModel, r.Model),
		attribute.Int(semconv.AttrMessagesCount, len(r.Contents)),
	}

	if r.GenerationConfig != nil {
		gc := r.GenerationConfig
		if gc.MaxOutputTokens > 0 {
			attrs = append(attrs, attribute.Int(semconv.AttrRequestMaxTokens, gc.MaxOutputTokens))
		}
		if gc.Temperature > 0 {
			attrs = append(attrs, attribute.Float64(semconv.AttrRequestTemperature, gc.Temperature))
		}
		if gc.TopP > 0 {
			attrs = append(attrs, attribute.Float64(semconv.AttrRequestTopP, gc.TopP))
		}
		if gc.TopK > 0 {
			attrs = append(attrs, attribute.Int(semconv.AttrRequestTopK, gc.TopK))
		}
		if len(gc.StopSequences) > 0 {
			attrs = append(attrs, attribute.StringSlice(semconv.AttrRequestStopSequences, gc.StopSequences))
		}
	}

	return attrs
}

// ExtractResponse extracts GenAI standard token usage and finish reason from a Gemini response.
func (p *GeminiProvider) ExtractResponse(resp any, err error) CallInfo {
	info := CallInfo{Provider: "gemini"}

	if err != nil {
		info.FinishReason = "error"
		return info
	}

	r, ok := resp.(GeminiResponse)
	if !ok {
		return info
	}

	info.Model = r.Model

	if r.UsageMetadata != nil {
		info.InputTokens = r.UsageMetadata.PromptTokenCount
		info.OutputTokens = r.UsageMetadata.CandidatesTokenCount
	}

	if len(r.Candidates) > 0 {
		info.FinishReason = r.Candidates[0].FinishReason
	}

	return info
}

// ExtractMessages extracts messages for opt-in content capture.
func (p *GeminiProvider) ExtractMessages(req any, resp any) (input []Message, output []Message) {
	r, ok := req.(GeminiRequest)
	if ok {
		for _, c := range r.Contents {
			content := ""
			for _, part := range c.Parts {
				content += part.Text
			}
			input = append(input, Message{Role: c.Role, Content: content})
		}
	}

	rr, ok := resp.(GeminiResponse)
	if ok {
		for _, cand := range rr.Candidates {
			content := ""
			for _, part := range cand.Content.Parts {
				content += part.Text
			}
			output = append(output, Message{Role: cand.Content.Role, Content: content})
		}
	}

	return
}

// Cost returns the estimated cost for a Gemini model call.
func (p *GeminiProvider) Cost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := geminiPricing[model]
	if !ok {
		return 0
	}
	inputCost := (float64(inputTokens) / 1_000_000) * pricing[0]
	outputCost := (float64(outputTokens) / 1_000_000) * pricing[1]
	return inputCost + outputCost
}

// SetGeminiPricing adds or overrides pricing for a Gemini model.
func SetGeminiPricing(model string, inputPer1M, outputPer1M float64) {
	geminiPricing[model] = [2]float64{inputPer1M, outputPer1M}
}

// FormatGeminiCost returns a human-readable cost string.
func FormatGeminiCost(dollars float64) string {
	if dollars < 0.01 {
		return fmt.Sprintf("$%.4f", dollars)
	}
	return fmt.Sprintf("$%.2f", dollars)
}
