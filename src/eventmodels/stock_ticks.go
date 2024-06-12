package eventmodels

import (
	"time"
)

type StockTicks []StockTickV1

func (ticks StockTicks) ToRows() [][]interface{} {
	results := make([][]interface{}, 0)

	for i := len(ticks) - 1; i >= 0; i-- {
		results = append(results, []interface{}{
			ticks[i].Timestamp.Format(time.RFC3339),
			ticks[i].LastPrice,
			ticks[i].Volume,
		})
	}

	return results
}
