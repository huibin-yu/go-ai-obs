package aiobs

import (
	"go.opentelemetry.io/otel/attribute"

	"github.com/yuhuibin/go-ai-obs/recorder"
)

// Config is the top-level configuration for go-ai-obs.
type Config struct {
	// ServiceName identifies this service in traces and metrics (required).
	ServiceName string

	// Environment is the deployment environment (e.g., "production", "staging").
	Environment string

	// CustomAttrs are static attributes added to every LLM span.
	CustomAttrs []attribute.KeyValue

	// SamplingRate controls trace sampling (0.0 to 1.0). Default: 1.0.
	SamplingRate float64

	// CaptureContent enables gen_ai.input.messages and gen_ai.output.messages.
	// Default: false (off for PII safety).
	CaptureContent bool
}

// Option configures go-ai-obs.
type Option func(*Config)

// WithServiceName sets the service name.
func WithServiceName(name string) Option {
	return func(c *Config) {
		c.ServiceName = name
	}
}

// WithEnvironment sets the deployment environment.
func WithEnvironment(env string) Option {
	return func(c *Config) {
		c.Environment = env
	}
}

// WithCustomAttr adds a static attribute to all LLM spans.
func WithCustomAttr(key, value string) Option {
	return func(c *Config) {
		c.CustomAttrs = append(c.CustomAttrs, attribute.String(key, value))
	}
}

// WithSamplingRate sets the trace sampling rate.
func WithSamplingRate(rate float64) Option {
	return func(c *Config) {
		c.SamplingRate = rate
	}
}

// WithCaptureContent enables capture of gen_ai.input.messages and gen_ai.output.messages.
// Use with caution — messages may contain sensitive data.
func WithCaptureContent() Option {
	return func(c *Config) {
		c.CaptureContent = true
	}
}

func (c *Config) toRecorderConfig() recorder.Config {
	return recorder.Config{
		ServiceName:    c.ServiceName,
		Environment:    c.Environment,
		CustomAttrs:    c.CustomAttrs,
		SamplingRate:   c.SamplingRate,
		CaptureContent: c.CaptureContent,
	}
}
