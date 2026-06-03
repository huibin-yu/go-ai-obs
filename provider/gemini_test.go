package provider

import (
	"errors"
	"testing"

	"github.com/yuhuibin/go-ai-obs/semconv"
)

func TestGeminiProvider_Name(t *testing.T) {
	p := NewGemini()
	if name := p.Name(); name != "gemini" {
		t.Errorf("expected 'gemini', got '%s'", name)
	}
}

func TestGeminiProvider_Operation(t *testing.T) {
	p := NewGemini()
	if op := p.Operation(); op != OpChat {
		t.Errorf("expected OpChat, got '%s'", op)
	}
}

func TestGeminiProvider_ExtractRequest(t *testing.T) {
	p := NewGemini()
	req := GeminiRequest{
		Model: "gemini-2.5-pro",
		Contents: []GeminiContent{
			{Role: "user", Parts: []GeminiPart{{Text: "Hello"}}},
		},
		GenerationConfig: &GeminiGenerationConfig{
			MaxOutputTokens: 512,
			Temperature:     0.8,
			TopP:            0.9,
			TopK:            40,
		},
	}

	attrs := p.ExtractRequest(req)
	m := attrsToMap(attrs)

	if v, ok := m[semconv.AttrRequestModel]; !ok || v.AsString() != "gemini-2.5-pro" {
		t.Errorf("expected model='gemini-2.5-pro', got %v", v.AsInterface())
	}
	if v, ok := m[semconv.AttrRequestMaxTokens]; !ok || v.AsInt64() != 512 {
		t.Errorf("expected max_tokens=512, got %v", v.AsInterface())
	}
	if v, ok := m[semconv.AttrRequestTopK]; !ok || v.AsInt64() != 40 {
		t.Errorf("expected top_k=40, got %v", v.AsInterface())
	}
}

func TestGeminiProvider_ExtractRequest_NoConfig(t *testing.T) {
	p := NewGemini()
	req := GeminiRequest{
		Model:    "gemini-2.0-flash",
		Contents: []GeminiContent{{Role: "user", Parts: []GeminiPart{{Text: "Hi"}}}},
	}

	attrs := p.ExtractRequest(req)
	m := attrsToMap(attrs)

	if v, ok := m[semconv.AttrMessagesCount]; !ok || v.AsInt64() != 1 {
		t.Errorf("expected messages_count=1, got %v", v.AsInterface())
	}
}

func TestGeminiProvider_ExtractRequest_InvalidType(t *testing.T) {
	p := NewGemini()
	if attrs := p.ExtractRequest("not a request"); attrs != nil {
		t.Errorf("expected nil for invalid type, got %v", attrs)
	}
}

func TestGeminiProvider_ExtractResponse_Success(t *testing.T) {
	p := NewGemini()
	resp := GeminiResponse{
		Model: "gemini-2.5-pro",
		Candidates: []GeminiCandidate{
			{
				FinishReason: "STOP",
				Content:      GeminiContent{Role: "model", Parts: []GeminiPart{{Text: "Hello!"}}},
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     100,
			CandidatesTokenCount: 50,
			TotalTokenCount:      150,
		},
	}

	info := p.ExtractResponse(resp, nil)

	if info.Model != "gemini-2.5-pro" {
		t.Errorf("expected model 'gemini-2.5-pro', got '%s'", info.Model)
	}
	if info.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", info.InputTokens)
	}
	if info.OutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", info.OutputTokens)
	}
	if info.FinishReason != "STOP" {
		t.Errorf("expected finish reason 'STOP', got '%s'", info.FinishReason)
	}
}

func TestGeminiProvider_ExtractResponse_Error(t *testing.T) {
	p := NewGemini()
	info := p.ExtractResponse(nil, errors.New("api error"))
	if info.FinishReason != "error" {
		t.Errorf("expected finish reason 'error', got '%s'", info.FinishReason)
	}
}

func TestGeminiProvider_ExtractMessages(t *testing.T) {
	p := NewGemini()
	req := GeminiRequest{
		Contents: []GeminiContent{
			{Role: "user", Parts: []GeminiPart{{Text: "Hello"}}},
		},
	}
	resp := GeminiResponse{
		Candidates: []GeminiCandidate{
			{Content: GeminiContent{Role: "model", Parts: []GeminiPart{{Text: "Hi!"}}}},
		},
	}

	input, output := p.ExtractMessages(req, resp)
	if len(input) != 1 || input[0].Content != "Hello" {
		t.Errorf("expected input 'Hello', got %v", input)
	}
	if len(output) != 1 || output[0].Content != "Hi!" {
		t.Errorf("expected output 'Hi!', got %v", output)
	}
}

func TestGeminiProvider_Cost(t *testing.T) {
	p := NewGemini()
	tests := []struct {
		model        string
		inputTokens  int
		outputTokens int
		expectedCost float64
	}{
		{"gemini-2.5-pro", 1_000_000, 0, 1.25},
		{"gemini-2.5-flash", 0, 1_000_000, 0.60},
		{"gemini-1.5-flash", 1000, 500, 0.000075 + 0.00015},
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
