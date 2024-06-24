package eventmodels

import "go.opentelemetry.io/otel/trace"

type TraceContext struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	TraceFlags trace.TraceFlags `json:"trace_flags"`
	TraceState string `json:"trace_state"` // serialized trace state if needed
}
