package provider

import (
	"math"

	"go.opentelemetry.io/otel/attribute"
)

// attrsToMap converts a slice of attribute.KeyValue to a map for easier testing.
func attrsToMap(attrs []attribute.KeyValue) map[string]attribute.Value {
	m := make(map[string]attribute.Value, len(attrs))
	for _, attr := range attrs {
		m[string(attr.Key)] = attr.Value
	}
	return m
}

// almostEqual compares two float64 values with a tolerance.
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}
