package provider

import (
	"errors"
	"testing"
)

func TestAnthropicProvider_Name(t *testing.T) {
	p := NewAnthropic()
	if name := p.Name(); name != "anthropic" {
		t.Errorf("expected 'anthropic', got '%s'", name)
	}
}

func TestAnthropicProvider_ExtractRequest(t *testing.T) {
	p := NewAnthropic()

	req := AnthropicRequest{
		Model:       "claude-sonnet-4-6",
		MaxTokens:   1024,
		Temperature: 0.5,
		TopP:        0.9,
		System:      "You are a helpful assistant.",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
		StopSequences: []string{"\n\nHuman:"},
	}

	attrs := p.ExtractRequest(req)
	m := attrsToMap(attrs)

	tests := []struct {
		key      string
		expected any
	}{
		{"llm.model", "claude-sonnet-4-6"},
		{"llm.max_tokens", int64(1024)},
		{"llm.messages_count", int64(2)},
		{"llm.has_system_prompt", true},
		{"llm.temperature", 0.5},
		{"llm.top_p", 0.9},
	}

	for _, tt := range tests {
		v, ok := m[tt.key]
		if !ok {
			t.Errorf("missing attribute: %s", tt.key)
			continue
		}
		val := v.AsInterface()
		if val != tt.expected {
			t.Errorf("%s: expected %v (%T), got %v (%T)", tt.key, tt.expected, tt.expected, val, val)
		}
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

	if _, ok := m["llm.has_system_prompt"]; ok {
		t.Error("should not have has_system_prompt when no system prompt")
	}
}

func TestAnthropicProvider_ExtractRequest_InvalidType(t *testing.T) {
	p := NewAnthropic()
	attrs := p.ExtractRequest("not a request")
	if attrs != nil {
		t.Errorf("expected nil for invalid type, got %v", attrs)
	}
}

func TestAnthropicProvider_ExtractResponse_Success(t *testing.T) {
	p := NewAnthropic()

	resp := AnthropicResponse{
		Model: "claude-sonnet-4-6-20250601",
		Usage: AnthropicUsage{
			InputTokens:  500,
			OutputTokens: 200,
		},
		StopReason: "end_turn",
	}

	info := p.ExtractResponse(resp, nil)

	if info.Model != "claude-sonnet-4-6-20250601" {
		t.Errorf("expected model 'claude-sonnet-4-6-20250601', got '%s'", info.Model)
	}
	if info.InputTokens != 500 {
		t.Errorf("expected 500 input tokens, got %d", info.InputTokens)
	}
	if info.OutputTokens != 200 {
		t.Errorf("expected 200 output tokens, got %d", info.OutputTokens)
	}
	if info.FinishReason != "end_turn" {
		t.Errorf("expected finish reason 'end_turn', got '%s'", info.FinishReason)
	}
}

func TestAnthropicProvider_ExtractResponse_Error(t *testing.T) {
	p := NewAnthropic()
	info := p.ExtractResponse(nil, errors.New("rate limit"))

	if info.FinishReason != "error" {
		t.Errorf("expected finish reason 'error', got '%s'", info.FinishReason)
	}
}

func TestAnthropicProvider_ExtractResponse_InvalidType(t *testing.T) {
	p := NewAnthropic()
	info := p.ExtractResponse("not a response", nil)

	if info.Model != "" {
		t.Errorf("expected empty model for invalid type, got '%s'", info.Model)
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
		{"claude-opus-4-8", 500_000, 250_000, 7.50 + 18.75}, // (0.5M * 15) + (0.25M * 75)
		{"claude-haiku-4-5", 1000, 500, 0.0008 + 0.002},
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

func TestSetAnthropicPricing(t *testing.T) {
	SetAnthropicPricing("test-model", 5.0, 10.0)
	p := NewAnthropic()
	cost := p.Cost("test-model", 1_000_000, 500_000)
	expected := 5.0 + 5.0 // (1M/1M * 5.0) + (500k/1M * 10.0)
	if !almostEqual(cost, expected, 0.0001) {
		t.Errorf("expected %.4f, got %.4f", expected, cost)
	}
}
