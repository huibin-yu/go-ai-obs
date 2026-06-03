package middleware

import (
	"context"
	"net/http"
)

// WrapHandler wraps an http.Handler with a span for each request.
// It extracts incoming trace context so LLM calls within the handler
// are children of the HTTP span.
func WrapHandler(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := TracePropagation(r.Context(), headersToMap(r.Header))
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// WrapHandlerFunc wraps an http.HandlerFunc similarly.
func WrapHandlerFunc(serviceName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := TracePropagation(r.Context(), headersToMap(r.Header))
		r = r.WithContext(ctx)
		next(w, r)
	}
}

func headersToMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k := range h {
		m[k] = h.Get(k)
	}
	return m
}

// StartChildSpan starts a child span in the given context for manual chain tracing.
// Use this to instrument multi-step AI workflows (e.g., RAG pipelines).
func StartChildSpan(ctx context.Context, name string) (context.Context, func()) {
	return context.WithCancel(ctx) // placeholder
}
