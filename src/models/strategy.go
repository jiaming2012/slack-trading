package models

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"sync"
	"time"
)

type Strategy struct {
	Name        string
	Conditions  []Condition
	Balance     float64
	PriceLevels *PriceLevels
	Symbol      string
	Direction   Direction
	mutex       sync.Mutex
}

// todo: modify to allow adding price level to strategy after creation
func (s *Strategy) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("validate: strategy must have a name")
	}

	if s.Symbol == "" {
		return fmt.Errorf("validate: strategy must have a symbol")
	}

	if s.Direction != Up && s.Direction != Down {
		return fmt.Errorf("validate: strategy direction must be either up or down")
	}

	if err := s.PriceLevels.Validate(); err != nil {
		return fmt.Errorf("validate: strategy initiated with invalid price levels: %w", err)
	}

	return nil
}

func (s *Strategy) GetPriceLevelByIndex(index int) (*PriceLevel, error) {
	return s.PriceLevels.GetByIndex(index)
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
	return s.Name
}

// todo: test this
func (s *Strategy) NewCloseTrade(id uuid.UUID, priceLevelIndex int, timeframe int, timestamp time.Time, requestedPrice float64, percent float64) (*Trade, error) {
	if percent < 0 || percent > 1 {
		return nil, InvalidClosePercentErr
	}

	priceLevel, err := s.GetPriceLevelByIndex(priceLevelIndex)
	if err != nil {
		return nil, fmt.Errorf("Strategy.NewOpenTrade: failed to get price level by index: %w", err)
	}

	openTrades := *priceLevel.Trades.OpenTrades()
	_, openTradesVol, _ := openTrades.Vwap()
	startingVolumeToClose := math.Abs(float64(openTradesVol)) * percent
	volumeToClose := startingVolumeToClose

	trades := make([]*Trade, 0)
	for _, tr := range openTrades {
		trades = append(trades, tr)
		closeVol := math.Min(volumeToClose, math.Abs(tr.ExecutedVolume))
		volumeToClose -= closeVol
		if volumeToClose == 0 {
			break
		}
	}

	closeVol := startingVolumeToClose - volumeToClose

	return NewCloseTrade(id, trades, timeframe, timestamp, requestedPrice, closeVol)
}

// todo: test this
func (s *Strategy) NewOpenTrade(id uuid.UUID, priceLevelIndex int, timeframe int, timestamp time.Time, requestedPrice float64) (*Trade, error) {
	priceLevel, err := s.GetPriceLevelByIndex(priceLevelIndex)
	if err != nil {
		return nil, fmt.Errorf("Strategy.NewOpenTrade: failed to get price level by index: %w", err)
	}

	maxLoss := s.Balance * priceLevel.AllocationPercent
	currentRisk, realizedPL := priceLevel.Trades.MaxRisk(priceLevel.StopLoss)
	tradesRemaining, _ := priceLevel.NewTradesRemaining()
	remainingRisk := (maxLoss + float64(realizedPL) - currentRisk) / float64(tradesRemaining)

	if remainingRisk < 0 {
		return nil, fmt.Errorf("Strategy.NewOpenTrade: remainingRisk = %v: %w", remainingRisk, NoRemainingRiskAvailable)
	}

	requestedVolume := remainingRisk / math.Abs(requestedPrice-priceLevel.StopLoss)

	return NewOpenTrade(id, s.GetTradeType(), s.Symbol, timeframe, timestamp, requestedPrice, requestedVolume, priceLevel.StopLoss)
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

func (s *Strategy) AutoExecuteTrade(trade *Trade) error {
	return s.ExecuteOpenTradeRequest(trade, trade.RequestedPrice, trade.RequestedVolume)
}

func (s *Strategy) ExecuteOpenTradeRequest(trade *Trade, price float64, volume float64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := trade.Validate(); err != nil {
		return fmt.Errorf("ExecuteTradeRequest failed to Validate trade: %w", err)
	}

	if err := s.CanPlaceTrade(trade); err != nil {
		return fmt.Errorf("ExecuteTradeRequest cannot place trade: %w", err)
	}

	trade.ExecutedPrice = price
	trade.ExecutedVolume = volume

	// todo: consider scenario where slippage causes a trade to be executed in a different price level
	var reqPrice float64
	if trade.Type == TradeTypeClose {
		if trade.Offsets == nil || len(trade.Offsets) == 0 {
			return fmt.Errorf("ExecuteOpenTradeRequest: closing trade does not have an offset trade")
		}

		reqPrice = trade.Offsets[0].RequestedPrice
	} else {
		reqPrice = trade.RequestedPrice
	}

	priceLevel := s.findPriceLevel(reqPrice)
	if priceLevel == nil {
		return fmt.Errorf("ExecuteTradeRequest failed to findPriceLevel at %.2f", trade.RequestedPrice)
	}

	priceLevel.Trades.Add(trade)

	return nil
}

func (s *Strategy) CanPlaceTrade2(tradeReq OpenTradeRequest) error {
	priceLevel := s.findPriceLevel(tradeReq.Price)

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

func (s *Strategy) CanPlaceTrade(trade *Trade) error {
	if trade.Type == TradeTypeClose {
		return nil
	}

	priceLevel := s.findPriceLevel(trade.RequestedPrice)
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

	strategy := &Strategy{
		Name:        name,
		Symbol:      symbol,
		Direction:   direction,
		Conditions:  make([]Condition, 0),
		Balance:     balance,
		PriceLevels: priceLevels,
	}

	if err = strategy.Validate(); err != nil {
		return nil, fmt.Errorf("NewStrategy: validation failed: %w", err)
	}

	return strategy, nil
}
