package models

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type CloseTradesRequest []CloseTradeRequest

type CloseTradeRequest struct {
	Trade      *Trade
	Reason     string
	Percentage float64
}

type BulkCloseRequestItem struct {
	Level        *PriceLevel
	ClosePercent float64
}

type BulkCloseRequest struct {
	Items []BulkCloseRequestItem
}

func (r *BulkCloseRequest) Execute(price float64, symbol string) ([]*Trade, error) {
	trades := make([]*Trade, 0)
	for _, it := range r.Items {
		if it.ClosePercent < 0 || it.ClosePercent > 1 {
			return nil, InvalidClosePercentErr
		}

		if it.Level.Trades != nil {
			_, vol, _ := it.Level.Trades.Vwap()
			closeVol := float64(vol) * it.ClosePercent * -1
			newTrade, err := NewTrade(uuid.New(), TradeTypeClose, symbol, time.Now(), price, closeVol, 0)
			if err != nil {
				return nil, fmt.Errorf("BulkCloseRequest.Execute: failed to open NewTrade: %w", err)
			}

			newTrade.RequestedVolume = closeVol

			it.Level.Trades.Add(newTrade)
			trades = append(trades, newTrade)
		}
	}

	return trades, nil
}

type OpenTradeRequest struct {
	Symbol   string
	Volume   float64
	Type     TradeType
	Price    float64
	StopLoss float64
	Strategy *Strategy
}
