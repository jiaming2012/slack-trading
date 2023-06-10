package models

type CloseTradeRequest struct {
	Trades Trades
}

type BulkCloseRequestItem struct {
	Level        *PriceLevel
	ClosePercent float64
}

type BulkCloseRequest struct {
	Items []BulkCloseRequestItem
}

func (r *BulkCloseRequest) Execute(price float64) ([]*Trade, error) {
	trades := make([]*Trade, 0)
	for _, it := range r.Items {
		if it.ClosePercent < 0 || it.ClosePercent > 1 {
			return nil, InvalidClosePercentErr
		}

		if it.Level.Trades != nil {
			_, vol, _ := it.Level.Trades.Vwap()
			closeVol := float64(vol) * it.ClosePercent * -1
			newTrade := NewTrade(price)
			newTrade.Volume = closeVol

			it.Level.Trades.Add(newTrade)
			trades = append(trades, newTrade)
		}
	}

	return trades, nil
}
