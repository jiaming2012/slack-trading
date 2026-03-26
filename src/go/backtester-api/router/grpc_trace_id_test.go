package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
	pb "github.com/jiaming2012/slack-trading/src/go/playground"
)

// setupTestTracer creates an in-memory span exporter and tracer provider for testing.
// Returns the exporter (for inspecting spans) and a cleanup function.
func setupTestTracer() (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	return exporter, tp
}

// ctxWithSpan creates a context with an active span from the given tracer provider.
func ctxWithSpan(tp *sdktrace.TracerProvider, spanName string) (context.Context, trace.Span) {
	tracer := tp.Tracer("test")
	return tracer.Start(context.Background(), spanName)
}

// findSpanAttribute searches exported spans for an attribute with the given key.
func findSpanAttribute(exporter *tracetest.InMemoryExporter, key string) (attribute.Value, bool) {
	spans := exporter.GetSpans()
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == key {
				return attr.Value, true
			}
		}
	}
	return attribute.Value{}, false
}

func TestNextTick_TraceIdAppearsInSpan_Success(t *testing.T) {
	exporter, tp := setupTestTracer()
	defer tp.Shutdown(context.Background())

	// Create a server with nil dbService -- NextTick will error when parsing playground_id
	// but the span attribute should be set before that point.
	// For a "success" test we need a valid UUID but no DB, so it will error at GetPlayground.
	// The trace_id span attribute is set before any DB call.
	s := &Server{cache: models.NewRequestCache(), dbService: nil}

	ctx, span := ctxWithSpan(tp, "NextTick")

	req := &pb.NextTickRequest{
		PlaygroundId: "550e8400-e29b-41d4-a716-446655440000",
		RequestId:    "req-001",
		TraceId:      "abc123def456",
		Seconds:      1,
	}

	// The call will panic or error because cache/dbService is nil,
	// but span attribute is set before those calls.
	// We need to recover from potential panics.
	func() {
		defer func() { recover() }()
		s.NextTick(ctx, req)
	}()

	span.End()

	val, found := findSpanAttribute(exporter, "trace_id")
	assert.True(t, found, "trace_id attribute should be present on span")
	assert.Equal(t, "abc123def456", val.AsString())
}

func TestPlaceOrder_TraceIdAppearsInSpan_Success(t *testing.T) {
	exporter, tp := setupTestTracer()
	defer tp.Shutdown(context.Background())

	s := &Server{cache: models.NewRequestCache(), dbService: nil}

	ctx, span := ctxWithSpan(tp, "PlaceOrder")

	req := &pb.PlaceOrderRequest{
		PlaygroundId: "550e8400-e29b-41d4-a716-446655440000",
		Symbol:       "AAPL",
		Side:         "buy",
		Quantity:     1.0,
		TraceId:      "xyz789",
	}

	func() {
		defer func() { recover() }()
		s.PlaceOrder(ctx, req)
	}()

	span.End()

	val, found := findSpanAttribute(exporter, "trace_id")
	assert.True(t, found, "trace_id attribute should be present on span")
	assert.Equal(t, "xyz789", val.AsString())
}

func TestNextTick_TraceIdAppearsInSpan_Error(t *testing.T) {
	exporter, tp := setupTestTracer()
	defer tp.Shutdown(context.Background())

	s := &Server{cache: models.NewRequestCache(), dbService: nil}

	ctx, span := ctxWithSpan(tp, "NextTick")

	req := &pb.NextTickRequest{
		PlaygroundId: "invalid-uuid",
		RequestId:    "req-err-001",
		TraceId:      "err-trace-001",
		Seconds:      1,
	}

	func() {
		defer func() { recover() }()
		s.NextTick(ctx, req)
	}()

	span.End()

	val, found := findSpanAttribute(exporter, "trace_id")
	assert.True(t, found, "trace_id attribute should be present on span even on error")
	assert.Equal(t, "err-trace-001", val.AsString())
}

func TestPlaceOrder_TraceIdAppearsInSpan_Error(t *testing.T) {
	exporter, tp := setupTestTracer()
	defer tp.Shutdown(context.Background())

	s := &Server{cache: models.NewRequestCache(), dbService: nil}

	ctx, span := ctxWithSpan(tp, "PlaceOrder")

	req := &pb.PlaceOrderRequest{
		PlaygroundId: "invalid-uuid",
		Symbol:       "AAPL",
		Side:         "buy",
		Quantity:     1.0,
		TraceId:      "err-trace-002",
	}

	func() {
		defer func() { recover() }()
		s.PlaceOrder(ctx, req)
	}()

	span.End()

	val, found := findSpanAttribute(exporter, "trace_id")
	assert.True(t, found, "trace_id attribute should be present on span even on error")
	assert.Equal(t, "err-trace-002", val.AsString())
}

func TestNextTick_EmptyTraceId_NoSpanAttribute(t *testing.T) {
	exporter, tp := setupTestTracer()
	defer tp.Shutdown(context.Background())

	s := &Server{cache: models.NewRequestCache(), dbService: nil}

	ctx, span := ctxWithSpan(tp, "NextTick")

	req := &pb.NextTickRequest{
		PlaygroundId: "invalid-uuid",
		RequestId:    "req-empty-001",
		TraceId:      "",
		Seconds:      1,
	}

	func() {
		defer func() { recover() }()
		s.NextTick(ctx, req)
	}()

	span.End()

	_, found := findSpanAttribute(exporter, "trace_id")
	assert.False(t, found, "trace_id attribute should NOT be present when trace_id is empty")
}
