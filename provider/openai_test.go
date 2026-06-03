package provider

import (
	"errors"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"github.com/yuhuibin/go-ai-obs/semconv"
)

func TestOpenAIProvider_Name(t *testing.T) {
	p := NewOpenAI()
	if name := p.Name(); name != "openai" {
		t.Errorf("expected 'openai', got '%s'", name)
	}
}

func TestOpenAIProvider_Operation(t *testing.T) {
	p := NewOpenAI()
	if op := p.Operation(); op != OpChat {
		t.Errorf("expected OpChat, got '%s'", op)
	}
}

func TestOpenAIProvider_ExtractRequest(t *testing.T) {
	p := NewOpenAI()

	req := openai.ChatCompletionRequest{
		Model:       "gpt-4o",
		Temperature: 0.7,
		MaxTokens:   256,
		TopP:        0.9,
		Stop:        []string{"\n"},
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
	}

	attrs := p.ExtractRequest(req)
	m := attrsToMap(attrs)

	// Check GenAI standard attribute keys
	if v, ok := m[semconv.AttrRequestModel]; !ok || v.AsString() != "gpt-4o" {
		t.Errorf("expected gen_ai.request.model='gpt-4o', got %v", v.AsInterface())
	}
	if v, ok := m[semconv.AttrRequestMaxTokens]; !ok {
		t.Errorf("missing %s", semconv.AttrRequestMaxTokens)
	} else if n := v.AsInt64(); n != 256 {
		t.Errorf("expected max_tokens=256, got %d", n)
	}
	if v, ok := m[semconv.AttrMessagesCount]; !ok {
		t.Errorf("missing %s", semconv.AttrMessagesCount)
	} else if n := v.AsInt64(); n != 2 {
		t.Errorf("expected messages_count=2, got %d", n)
	}

	// Float values need tolerance checks (float32→float64 conversion)
	if v, ok := m[semconv.AttrRequestTemperature]; ok {
		f, _ := v.AsInterface().(float64)
		if !almostEqual(f, 0.7, 0.001) {
			t.Errorf("temperature: expected ~0.7, got %v", f)
		}
	}
	if v, ok := m[semconv.AttrRequestTopP]; ok {
		f, _ := v.AsInterface().(float64)
		if !almostEqual(f, 0.9, 0.001) {
			t.Errorf("top_p: expected ~0.9, got %v", f)
		}
	}
}

func TestOpenAIProvider_ExtractRequest_InvalidType(t *testing.T) {
	p := NewOpenAI()
	attrs := p.ExtractRequest("not a request")
	if attrs != nil {
		t.Errorf("expected nil for invalid type, got %v", attrs)
	}
}

func TestOpenAIProvider_ExtractResponse_Success(t *testing.T) {
	p := NewOpenAI()

	resp := openai.ChatCompletionResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4o-2024-05-13",
		Usage: openai.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Choices: []openai.ChatCompletionChoice{
			{
				FinishReason: openai.FinishReasonStop,
				Message: openai.ChatCompletionMessage{Content: "Hello!"},
			},
		},
	}

	info := p.ExtractResponse(resp, nil)

	if info.Model != "gpt-4o-2024-05-13" {
		t.Errorf("expected model 'gpt-4o-2024-05-13', got '%s'", info.Model)
	}
	if info.ResponseID != "chatcmpl-123" {
		t.Errorf("expected response ID 'chatcmpl-123', got '%s'", info.ResponseID)
	}
	if info.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", info.InputTokens)
	}
	if info.OutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", info.OutputTokens)
	}
	if info.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got '%s'", info.FinishReason)
	}
}

func TestOpenAIProvider_ExtractResponse_Error(t *testing.T) {
	p := NewOpenAI()
	info := p.ExtractResponse(nil, errors.New("api error"))
	if info.FinishReason != "error" {
		t.Errorf("expected finish reason 'error', got '%s'", info.FinishReason)
	}
}

func TestOpenAIProvider_ExtractResponse_InvalidType(t *testing.T) {
	p := NewOpenAI()
	info := p.ExtractResponse("not a response", nil)
	if info.Model != "" {
		t.Errorf("expected empty model, got '%s'", info.Model)
	}
}

func TestOpenAIProvider_ExtractMessages(t *testing.T) {
	p := NewOpenAI()
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Role: "assistant", Content: "Hi!"}},
		},
	}

	input, output := p.ExtractMessages(req, resp)
	if len(input) != 1 || input[0].Content != "Hello" {
		t.Errorf("expected 1 input message 'Hello', got %v", input)
	}
	if len(output) != 1 || output[0].Content != "Hi!" {
		t.Errorf("expected 1 output message 'Hi!', got %v", output)
	}
}

func TestOpenAIProvider_Cost(t *testing.T) {
	p := NewOpenAI()
	tests := []struct {
		model        string
		inputTokens  int
		outputTokens int
		expectedCost float64
	}{
		{"gpt-4o", 1_000_000, 0, 2.50},
		{"gpt-4o", 0, 1_000_000, 10.00},
		{"gpt-4o", 1000, 1000, 0.0025 + 0.010},
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

func TestRegisterPricing(t *testing.T) {
	RegisterPricing("test-model", 1.0, 2.0)
	p := NewOpenAI()
	cost := p.Cost("test-model", 1_000_000, 500_000)
	if !almostEqual(cost, 2.0, 0.0001) {
		t.Errorf("expected 2.0, got %.4f", cost)
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		dollars  float64
		expected string
	}{
		{1.50, "$1.50"},
		{0.001, "$0.0010"},
		{0.0, "$0.0000"},
	}
	for _, tt := range tests {
		if got := FormatCost(tt.dollars); got != tt.expected {
			t.Errorf("FormatCost(%.4f): expected %s, got %s", tt.dollars, tt.expected, got)
		}
	}
}
