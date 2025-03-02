package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type IPolygonClient interface {
	FetchAggregateBars(ticker eventmodels.Instrument, timespan eventmodels.PolygonTimespan, from, to *eventmodels.PolygonDate) ([]*eventmodels.PolygonAggregateBarV2, error)
	FetchPastCandles(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, daysPast int, end *eventmodels.PolygonDate) ([]*eventmodels.PolygonAggregateBarV2, error)
}
