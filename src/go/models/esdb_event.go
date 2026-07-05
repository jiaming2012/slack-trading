package models

import "go.opencensus.io/trace"

type EsdbEvent[T SavedEvent] struct {
	Event       T
	IsReplay    bool
	SpanContext trace.SpanContext
}
