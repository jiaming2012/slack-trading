package models

import "github.com/jiaming2012/slack-trading/src/go/models"

type IPolygonClient interface {
	FetchAggregateBars(ticker models.Instrument, timespan models.PolygonTimespan, from, to *models.PolygonDate) ([]*models.PolygonAggregateBarV2, error)
	FetchPastCandles(symbol models.StockSymbol, timespan models.PolygonTimespan, daysPast int, end *models.PolygonDate) ([]*models.PolygonAggregateBarV2, error)
}
