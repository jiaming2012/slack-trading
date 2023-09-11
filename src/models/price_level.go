package models

import (
	"fmt"
	"math"
	"sync"
)

const SmallRoundingError = 0.00000001

type PriceLevel struct {
	Price                float64
	MinimumTradeDistance float64 // the minimum distance of the requested price of two trades in the same price band
	MaxNoOfTrades        int
	AllocationPercent    float64 // the amount of Account.Balance allocated to this price level
	Trades               *Trades
	StopLoss             float64
	mutex                sync.Mutex
}

func (p *PriceLevel) canAddTrade(trade *Trade) error {
	if trade.Type == TradeTypeBuy || trade.Type == TradeTypeSell {
		openTrades := p.Trades.OpenTrades()
		for _, open := range *openTrades {
			t := math.Abs(trade.RequestedPrice-open.RequestedPrice) + SmallRoundingError
			if t < p.MinimumTradeDistance {
				return fmt.Errorf("PriceLevel.canAddTrade: request price of %v is too close to request price of previously open trade %v with minimum distance of %v: %w", trade.RequestedPrice, open.RequestedPrice, p.MinimumTradeDistance, PriceLevelMinimumDistanceNotSatisfiedError)
			}
		}
	}

	return nil
}

func (p *PriceLevel) Add(trade *Trade) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.canAddTrade(trade); err != nil {
		return err
	}

	p.Trades.Add(trade)

	return nil
}

func (p *PriceLevel) Validate() error {
	if p.MinimumTradeDistance < 0 {
		return PriceLevelMinimumDistanceNotSatisfiedError
	}

	if p.AllocationPercent < 0 || p.AllocationPercent > 1 {
		return InvalidAllocationPercentErr
	}

	if p.AllocationPercent > 0 && p.StopLoss <= 0 {
		return NonPositiveStopLoss
	}

	if p.MaxNoOfTrades < 0 {
		return InvalidMaxTradesErr
	}

	if p.Price < 0 {
		return NegativePriceErr
	}

	return nil
}

func (p *PriceLevel) NewTradesRemaining() (int, TradeType) {
	buysCount := 0
	buyVolume := 0.0
	sellVolume := 0.0
	sellsCount := 0
	closedBuyVolume := 0.0
	closedSellVolume := 0.0
	diff := 0

	for _, t := range *p.Trades {
		if t.Type == TradeTypeClose {
			if t.ExecutedVolume < 0 {
				closedBuyVolume += math.Abs(t.ExecutedVolume)
			} else if t.ExecutedVolume > 0 {
				closedSellVolume += t.ExecutedVolume
			}
		}
	}

	// todo: null pointer check
	for _, t := range *p.Trades {
		if t.Type == TradeTypeBuy {
			executedVolume := t.ExecutedVolume
			if closedBuyVolume > 0 {
				executedVolume -= math.Min(t.ExecutedVolume, closedBuyVolume)
				closedBuyVolume = math.Max(closedBuyVolume-t.ExecutedVolume, 0)
			}

			if executedVolume > 0 {
				buysCount += 1
				buyVolume += executedVolume
			}
		} else if t.Type == TradeTypeSell {
			executedVolume := math.Abs(t.ExecutedVolume)
			if closedSellVolume > 0 {
				executedVolume -= math.Min(t.ExecutedVolume, closedBuyVolume)
				closedSellVolume = math.Max(closedBuyVolume-t.ExecutedVolume, 0)
			}

			if executedVolume > 0 {
				sellsCount += 1
				sellVolume += executedVolume
			}
		}
	}

	var side TradeType
	if buysCount > 0 {
		side = TradeTypeBuy
		diff = buysCount
	} else if sellsCount > 0 {
		side = TradeTypeSell
		diff = sellsCount
	} else {
		side = TradeTypeNone
		diff = 0
	}

	return p.MaxNoOfTrades - diff, side
}
