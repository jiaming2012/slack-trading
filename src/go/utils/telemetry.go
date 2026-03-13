package utils

import (
	"encoding/json"

	"go.opentelemetry.io/otel/trace"
)

type TraceContextDTO struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	TraceFlags byte   `json:"trace_flags"`
	TraceState string `json:"trace_state"` // serialized trace state if needed
	IsRemote   bool   `json:"is_remote"`
}

func SerializeTraceContext(sc trace.SpanContext) ([]byte, error) {
	traceContext := TraceContextDTO{
		TraceID:    sc.TraceID().String(),
		SpanID:     sc.SpanID().String(),
		TraceFlags: byte(sc.TraceFlags()),
		TraceState: sc.TraceState().String(),
		IsRemote:   sc.IsRemote(),
	}

	return json.Marshal(traceContext)
}

func DeserializeTraceContext(data []byte) (trace.SpanContext, error) {
	var traceContext TraceContextDTO
	err := json.Unmarshal([]byte(data), &traceContext)
	if err != nil {
		return trace.SpanContext{}, err
	}

	traceID, err := trace.TraceIDFromHex(traceContext.TraceID)
	if err != nil {
		return trace.SpanContext{}, err
	}

	spanID, err := trace.SpanIDFromHex(traceContext.SpanID)
	if err != nil {
		return trace.SpanContext{}, err
	}

	traceState, err := trace.ParseTraceState(traceContext.TraceState)
	if err != nil {
		return trace.SpanContext{}, err
	}

	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.TraceFlags(traceContext.TraceFlags),
		TraceState: traceState,
		Remote:     traceContext.IsRemote,
	}), nil
}
