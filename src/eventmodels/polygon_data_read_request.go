package eventmodels

import (
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

type PolygonDataReadRequest struct {
	Symbol     StockSymbol
	From       time.Time
	To         time.Time
	Multiplier int
	Timespan   models.Timespan
}
