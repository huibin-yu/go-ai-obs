// Package middleware provides HTTP framework integrations for go-ai-obs.
package middleware

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/gin-gonic/gin"
)

// GinMiddleware returns a Gin middleware that creates a span for each HTTP request.
// It injects request context (method, path, status) and propagates trace context
// so LLM calls within the request are linked to the HTTP span.
//
// Usage:
//
//	r := gin.Default()
//	r.Use(middleware.GinMiddleware("my-service"))
func GinMiddleware(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer("go-ai-obs/http",
		// Note: version is set via the tracer provider, not here individually
	)

	propagator := propagation.TraceContext{}

	return func(c *gin.Context) {
		// Extract trace context from incoming headers
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		c.Request = c.Request.WithContext(ctx)

		// Start a span for this HTTP request
		ctx, span := tracer.Start(ctx, c.FullPath(),
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.url", c.Request.URL.String()),
				attribute.String("http.route", c.FullPath()),
			),
		)
		defer span.End()

		// Inject trace context back into request
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Record response attributes
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
		)

		if c.Writer.Status() >= 400 {
			span.SetStatus(codes.Error, "")
		}
	}
}

// TracePropagation returns a context with trace extracted from headers,
// suitable for generic HTTP handler wrapping.
func TracePropagation(ctx context.Context, headers map[string]string) context.Context {
	propagator := propagation.TraceContext{}
	carrier := propagation.MapCarrier(headers)
	return propagator.Extract(ctx, carrier)
}
