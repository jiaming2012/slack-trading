package models

import (
	"fmt"
	"strings"
)

type Strategy struct {
	Name        string
	Conditions  []Condition
	Balance     float64
	PriceLevels *PriceLevels
	Symbol      string
	Direction   Direction
}

func (s *Strategy) GetTrades() *Trades {
	trades := Trades{}

	for _, level := range s.PriceLevels.Values {
		for _, tr := range *level.Trades {
			trades = append(trades, tr)
		}
	}

	return &trades
}

func (s Strategy) String() string {
	str := strings.Builder{}

	str.WriteString(fmt.Sprintf("%v\n", s.Name))

	for _, signal := range s.Conditions {
		str.WriteString(signal.String() + "\n")
	}

	return str.String()
}

func (s *Strategy) TradesRemaining(price float64) (int, TradeType) {
	lvl := s.findPriceLevel(price)
	if lvl != nil {
		return lvl.NewTradesRemaining()
	}
	return 0, TradeTypeBuy
}

func (s *Strategy) findPriceLevel(price float64) *PriceLevel {
	for i, priceLevel := range s.PriceLevels.Values[:len(s.PriceLevels.Values)-1] {
		if price >= s.PriceLevels.Values[i].Price && price < s.PriceLevels.Values[i+1].Price {
			return priceLevel
		}
	}

	return nil
}

func (s *Strategy) isConditionUnique(signal Signal) bool {
	for _, cond := range s.Conditions {
		if cond.Signal.String() == signal.String() {
			return false
		}
	}

	return true
}

func (s *Strategy) RemoveCondition(signal Signal) error {
	for i, cond := range s.Conditions {
		if cond.Signal.String() == signal.String() {
			s.Conditions = append(s.Conditions[:i], s.Conditions[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("Strategy.RemoveCondition: could not find signal %v", signal)
}

func (s *Strategy) AddCondition(signal Signal) error {
	if !s.isConditionUnique(signal) {
		return fmt.Errorf("signal %v already exists", signal)
	}

	s.Conditions = append(s.Conditions, Condition{
		Signal:      signal,
		IsSatisfied: false,
	})

	return nil
}

func (s *Strategy) GetTradeType() TradeType {
	switch s.Direction {
	case Up:
		return TradeTypeBuy
	case Down:
		return TradeTypeSell
	default:
		return TradeTypeUnknown
	}
}

func (s *Strategy) AutoExecuteTradeRequest(trade *Trade) error {
	return s.ExecuteTradeRequest(trade, trade.RequestedPrice, trade.RequestedVolume)
}

func (s *Strategy) ExecuteTradeRequest(trade *Trade, price float64, volume float64) error {
	if err := trade.Validate(); err != nil {
		return fmt.Errorf("ExecuteTradeRequest failed to Validate trade: %w", err)
	}

	if err := s.CanPlaceTrade(trade.RequestedPrice); err != nil {
		return fmt.Errorf("ExecuteTradeRequest cannot place trade: %w", err)
	}

	trade.ExecutedPrice = price
	trade.ExecutedVolume = volume

	// todo: consider scenario where slippage causes a trade to be executed in a different price level
	priceLevel := s.findPriceLevel(trade.RequestedPrice)
	if priceLevel == nil {
		return fmt.Errorf("ExecuteTradeRequest failed to findPriceLevel at %.2f", trade.RequestedPrice)
	}

	priceLevel.Trades.Add(trade)

	return nil
}

func (s *Strategy) CanPlaceTrade(price float64) error {
	priceLevel := s.findPriceLevel(price)
	if priceLevel == nil {
		return PriceOutsideLimitsErr
	}

	if priceLevel.MaxNoOfTrades <= 0 {
		return MaxTradesPerPriceLevelErr
	}

	tradesRemaining, side := priceLevel.NewTradesRemaining()
	tradeType := s.GetTradeType()
	if tradeType == TradeTypeBuy {
		if side == TradeTypeBuy && tradesRemaining <= 0 {
			return MaxTradesPerPriceLevelErr
		}
	} else if tradeType == TradeTypeSell {
		if side == TradeTypeSell && tradesRemaining <= 0 {
			return MaxTradesPerPriceLevelErr
		}
	}

	_, _, realizedPL := priceLevel.Trades.Vwap()

	maxPriceLevelLoss := s.Balance * priceLevel.AllocationPercent
	maxTradeLoss := maxPriceLevelLoss / float64(priceLevel.MaxNoOfTrades)

	if float64(realizedPL)+maxTradeLoss > maxPriceLevelLoss {
		return MaxLossPriceBandErr
	}

	return nil
	//return &TradeParameters{
	//	PriceLevel: priceLevel,
	//	MaxLoss:    maxTradeLoss,
	//}, nil
}

func NewStrategy(name string, symbol string, direction Direction, balance float64, priceLevelInput []*PriceLevel) (*Strategy, error) {
	if balance <= 0 {
		return nil, BalanceGreaterThanZeroErr
	}

	priceLevels, err := NewPriceLevels(priceLevelInput)
	if err != nil {
		return nil, fmt.Errorf("NewStrategy: failed to create price levels: %w", err)
	}

	return &Strategy{
		Name:        name,
		Symbol:      symbol,
		Direction:   direction,
		Conditions:  make([]Condition, 0),
		Balance:     balance,
		PriceLevels: priceLevels,
	}, nil
}
