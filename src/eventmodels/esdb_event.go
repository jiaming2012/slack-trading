package eventmodels

import "go.opencensus.io/trace"

type EsdbEvent[T SavedEvent] struct {
	Event       T
	IsReplay    bool
	SpanContext trace.SpanContext
}
