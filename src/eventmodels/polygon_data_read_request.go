package eventmodels

import (
	"github.com/polygon-io/client-go/rest/models"
)

type PolygonDataReadRequest struct {
	Symbol     StockSymbol
	From       string
	To         string
	Multiplier int
	Timespan   models.Timespan
}
