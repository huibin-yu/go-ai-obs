// Package main demonstrates basic usage of go-ai-obs with the OpenAI provider.
//
// This example starts an OTLP exporter, wraps an OpenAI client, and makes a
// traced chat completion call. Run with a local Jaeger or Tempo instance:
//
//	docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest
//	go run _examples/basic/main.go
//
// Then open http://localhost:16686 to see the traces.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	openai "github.com/sashabaranov/go-openai"

	aiobs "github.com/yuhuibin/go-ai-obs"
	_ "github.com/yuhuibin/go-ai-obs/middleware"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Println("OPENAI_API_KEY not set — running in demo mode with a mock call")
		apiKey = "sk-demo-key"
	}

	// Step 1: Create the observability recorder.
	rec, err := aiobs.New(
		aiobs.WithServiceName("aiobs-demo"),
		aiobs.WithEnvironment("development"),
		aiobs.WithCustomAttr("demo", "true"),
	)
	if err != nil {
		log.Fatalf("Failed to create recorder: %v", err)
	}
	defer rec.Shutdown(context.Background())

	// Step 2: Expose Prometheus metrics on :9090/metrics
	go func() {
		http.Handle("/metrics", rec.MetricsHandler().Handler())
		log.Println("Metrics available at http://localhost:9090/metrics")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Printf("Metrics server: %v", err)
		}
	}()

	// Step 3: Wrap the OpenAI client for automatic tracing.
	rawClient := openai.NewClient(apiKey)
	client := rec.WrapOpenAI(rawClient)

	// Step 4: Make a traced call.
	ctx := context.Background()
	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Say 'Hello from go-ai-obs!' in exactly one short sentence.",
			},
		},
		MaxTokens:   50,
		Temperature: 0.7,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("LLM call failed (expected if no valid API key): %v", err)
		os.Exit(0)
	}

	fmt.Printf("Response: %s\n", resp.Choices[0].Message.Content)
	fmt.Printf("Model: %s\n", resp.Model)
	fmt.Printf("Tokens: %d prompt + %d completion = %d total\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	fmt.Println("Check Jaeger at http://localhost:16686 for the trace.")

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
