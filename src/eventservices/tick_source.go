package eventservices

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type TickSource interface {
	FetchAggregateBars(ticker eventmodels.Instrument, timespan eventmodels.PolygonTimespan, from, to *eventmodels.PolygonDate) ([]*eventmodels.PolygonAggregateBarV2, error)
}
