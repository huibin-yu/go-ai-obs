package provider

import (
	"errors"
	"testing"

	"github.com/yuhuibin/go-ai-obs/semconv"
)

func TestAnthropicProvider_Name(t *testing.T) {
	p := NewAnthropic()
	if name := p.Name(); name != "anthropic" {
		t.Errorf("expected 'anthropic', got '%s'", name)
	}
}

func TestAnthropicProvider_Operation(t *testing.T) {
	p := NewAnthropic()
	if op := p.Operation(); op != OpChat {
		t.Errorf("expected OpChat, got '%s'", op)
	}
}

func TestAnthropicProvider_ExtractRequest(t *testing.T) {
	p := NewAnthropic()
	req := AnthropicRequest{
		Model:       "claude-sonnet-4-6",
		MaxTokens:   1024,
		Temperature: 0.5,
		TopP:        0.9,
		TopK:        40,
		System:      "You are a helpful assistant.",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}

	attrs := p.ExtractRequest(req)
	m := attrsToMap(attrs)

	if v, ok := m[semconv.AttrRequestModel]; !ok || v.AsString() != "claude-sonnet-4-6" {
		t.Errorf("expected model='claude-sonnet-4-6', got %v", v.AsInterface())
	}
	if v, ok := m[semconv.AttrRequestMaxTokens]; !ok || v.AsInt64() != 1024 {
		t.Errorf("expected max_tokens=1024, got %v", v.AsInterface())
	}
	if v, ok := m[semconv.AttrMessagesCount]; !ok || v.AsInt64() != 2 {
		t.Errorf("expected messages_count=2, got %v", v.AsInterface())
	}
	if _, ok := m[semconv.AttrHasSystemPrompt]; !ok {
		t.Error("expected has_system_prompt=true")
	}
	if v, ok := m[semconv.AttrRequestTopK]; !ok || v.AsInt64() != 40 {
		t.Errorf("expected top_k=40, got %v", v.AsInterface())
	}
}

func TestAnthropicProvider_ExtractRequest_NoSystem(t *testing.T) {
	p := NewAnthropic()
	req := AnthropicRequest{
		Model: "claude-haiku-4-5",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	attrs := p.ExtractRequest(req)
	m := attrsToMap(attrs)
	if _, ok := m[semconv.AttrHasSystemPrompt]; ok {
		t.Error("should not have has_system_prompt when no system prompt")
	}
}

func TestAnthropicProvider_ExtractRequest_InvalidType(t *testing.T) {
	p := NewAnthropic()
	if attrs := p.ExtractRequest("not a request"); attrs != nil {
		t.Errorf("expected nil for invalid type, got %v", attrs)
	}
}

func TestAnthropicProvider_ExtractResponse_Success(t *testing.T) {
	p := NewAnthropic()
	resp := AnthropicResponse{
		ID:    "msg_123",
		Model: "claude-sonnet-4-6-20250601",
		Usage: AnthropicUsage{InputTokens: 500, OutputTokens: 200},
		StopReason: "end_turn",
	}
	info := p.ExtractResponse(resp, nil)

	if info.Model != "claude-sonnet-4-6-20250601" {
		t.Errorf("expected model 'claude-sonnet-4-6-20250601', got '%s'", info.Model)
	}
	if info.ResponseID != "msg_123" {
		t.Errorf("expected response ID 'msg_123', got '%s'", info.ResponseID)
	}
	if info.InputTokens != 500 {
		t.Errorf("expected 500 input tokens, got %d", info.InputTokens)
	}
}

func TestAnthropicProvider_ExtractResponse_Error(t *testing.T) {
	p := NewAnthropic()
	info := p.ExtractResponse(nil, errors.New("rate limit"))
	if info.FinishReason != "error" {
		t.Errorf("expected finish reason 'error', got '%s'", info.FinishReason)
	}
}

func TestAnthropicProvider_ExtractMessages(t *testing.T) {
	p := NewAnthropic()
	req := AnthropicRequest{
		System: "You are helpful.",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Hi"},
		},
	}
	input, output := p.ExtractMessages(req, nil)
	// System prompt should be prepended
	if len(input) != 2 || input[0].Role != "system" || input[1].Content != "Hi" {
		t.Errorf("expected 2 input messages with system prepended, got %v", input)
	}
	if output != nil {
		t.Errorf("expected nil output when no response")
	}
}

func TestAnthropicProvider_Cost(t *testing.T) {
	p := NewAnthropic()
	tests := []struct {
		model        string
		inputTokens  int
		outputTokens int
		expectedCost float64
	}{
		{"claude-sonnet-4-6", 1_000_000, 0, 3.00},
		{"claude-sonnet-4-6", 0, 1_000_000, 15.00},
		{"unknown-model", 1000, 1000, 0},
	}
	for _, tt := range tests {
		cost := p.Cost(tt.model, tt.inputTokens, tt.outputTokens)
		if !almostEqual(cost, tt.expectedCost, 0.0001) {
			t.Errorf("Cost(%s, %d, %d): expected %.6f, got %.6f",
				tt.model, tt.inputTokens, tt.outputTokens, tt.expectedCost, cost)
		}
	}
}
