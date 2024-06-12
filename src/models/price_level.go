package models

import (
	"fmt"
	"math"
	"sync"
)

const SmallRoundingError = 0.000001

type PriceLevelDTO struct {
	Price                float64 `json:"price"`
	MinimumTradeDistance float64 `json:"minimumTradeDistance"`
	MaxNoOfTrades        int     `json:"maxNoOfTrades"`
	AllocationPercent    float64 `json:"allocationPercent"`
	StopLoss             float64 `json:"stopLoss"`
}

func (dto *PriceLevelDTO) ToPriceLevel() *PriceLevel {
	return NewPriceLevel(dto.Price, dto.MinimumTradeDistance, dto.MaxNoOfTrades, dto.AllocationPercent, dto.StopLoss)
}

func NewPriceLevel(price float64, minimumTradeDistance float64, maxNoOfTrades int, allocationPercent float64, stopLoss float64) *PriceLevel {
	return &PriceLevel{
		Strategy:             nil,
		Trades:               &Trades{},
		mutex:                sync.Mutex{},
		Price:                price,
		MinimumTradeDistance: minimumTradeDistance,
		MaxNoOfTrades:        maxNoOfTrades,
		AllocationPercent:    allocationPercent,
		StopLoss:             stopLoss,
	}
}

type PriceLevel struct {
	Strategy *Strategy `json:"-"`
	// todo: add Index
	Price                float64    `json:"price"`
	MinimumTradeDistance float64    `json:"minimumTradeDistance"` // the minimum distance of the requested price of two trades in the same price band
	MaxNoOfTrades        int        `json:"maxNoOfTrades"`
	AllocationPercent    float64    `json:"allocationPercent"` // the amount of Account.Balance allocated to this price level
	Trades               *Trades    `json:"trades"`
	StopLoss             float64    `json:"stopLoss"`
	mutex                sync.Mutex `json:"-"`
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

func (p *PriceLevel) Add(trade *Trade, executedPrice float64, executedVolume float64) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.canAddTrade(trade); err != nil {
		return err
	}

	//partialCloseItems, err := trade.PreparePartialCloseItems(executedPrice, executedVolume)
	//if err != nil {
	//	return fmt.Errorf("Trade.Execute: failed to modify trades to add partial close items: %w", err)
	//}

	//if err = trade.Validate(partialCloseItems); err != nil {
	//	return fmt.Errorf("PriceLevel.Add: trade is not valid: %w", err)
	//}

	if err := trade.Execute(executedPrice, executedVolume); err != nil {
		return fmt.Errorf("PriceLevel.Add: failed to execute trade: %w", err)
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
	// todo: make this part of GetTradeStats, along with GetOpenTrades
	openTrades := p.Trades.OpenTrades()

	if openTrades == nil || len(*openTrades) == 0 {
		return p.MaxNoOfTrades, TradeTypeNone
	}

	remaining := math.Max(float64(p.MaxNoOfTrades-len(*openTrades)), 0)

	return int(remaining), (*openTrades)[0].Type
}
