package models

import (
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"math"
	"sync"
	"time"
)

type Strategy struct {
	Name                         string            `json:"name"`
	EntryConditions              []*EntryCondition `json:"entryConditions"`
	ExitConditions               []*ExitCondition  `json:"exitConditions"`
	Balance                      float64           `json:"balance"`
	PriceLevels                  *PriceLevels      `json:"priceLevels"`
	Symbol                       string            `json:"symbol"`
	Direction                    Direction         `json:"direction"`
	Account                      *Account          `json:"-"`
	getTradesMutex               sync.Mutex        `json:"-"`
	executeOpenTradeRequestMutex sync.Mutex        `json:"-"`
}

// Validate todo: modify to allow adding price level to strategy after creation
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

	return nil
}

// UpdateExitConditions todo: test this
func (s *Strategy) UpdateExitConditions(signalName string) int {
	conditionsAffected := 0

	for _, condition := range s.ExitConditions {
		for _, exitSignal := range condition.ExitSignals {
			//--- update exit signal
			if signalName == exitSignal.Signal.Name {
				exitSignal.Update(SignalTypeExit)
				conditionsAffected += 1
				log.Infof("setting exit condition %v to true", signalName)
			} else if signalName == exitSignal.ResetSignal.Name {
				exitSignal.Update(SignalTypeReset)
				conditionsAffected += 1
				log.Infof("setting exit reset condition %v to true", signalName)
			}

			//--- update reentry signals
			for _, reentrySignal := range condition.ReentrySignals {
				if signalName == reentrySignal.Name {
					reentrySignal.IsSatisfied = true
				}
			}
		}
	}

	return conditionsAffected
}

func (s *Strategy) UpdateEntryConditions(signalName string) int {
	conditionsAffected := 0

	for _, condition := range s.EntryConditions {
		if signalName == condition.EntrySignal.Name {
			condition.UpdateState(true)
			conditionsAffected += 1
			log.Infof("setting entry condition %v to true", signalName)
		} else if signalName == condition.ResetSignal.Name {
			condition.UpdateState(false)
			conditionsAffected += 1
			log.Infof("setting exit condition %v to true", signalName)
		}
	}

	return conditionsAffected
}

func (s *Strategy) ExitConditionsSatisfied(tick Tick) ([]*ExitConditionsSatisfied, error) {
	if len(s.ExitConditions) == 0 {
		log.Warnf("ExitConditionsSatisfied for Strategy %v will never return true until at least one entry condition is added", s)
		return nil, nil
	}

	var exitConditionsSatisfied []*ExitConditionsSatisfied
	params := map[string]interface{}{"tick": tick}
	for levelIndex, level := range s.PriceLevels.Bands {
		var exitCondition *ExitCondition
		for _, cond := range s.ExitConditions {
			if cond.LevelIndex != levelIndex {
				// todo: handle this inside of cond.IsSatisfied once price levels has an index attribute
				continue
			}
			isSatisfied, err := cond.IsSatisfied(level, params)
			if err != nil {
				return nil, fmt.Errorf("ExitConditionsSatisfied: condition check failed: %w", err)
			}

			if isSatisfied {
				exitCondition = cond
				break
			}
		}

		if exitCondition != nil {
			exitConditionsSatisfied = append(exitConditionsSatisfied, &ExitConditionsSatisfied{
				PriceLevel:      level,
				PriceLevelIndex: levelIndex,
				PercentClose:    exitCondition.ClosePercent,
				Reason:          exitCondition.Name,
			})
		}
	}

	return exitConditionsSatisfied, nil
}

func (s *Strategy) EntryConditionsSatisfied() bool {
	if len(s.EntryConditions) == 0 {
		log.Warnf("EntryConditionsSatisfied for Strategy %v will never return true until at least one entry condition is added", s)
		return false
	}

	for _, cond := range s.EntryConditions {
		if !cond.EntrySignal.IsSatisfied || cond.ResetSignal.IsSatisfied {
			return false
		}
	}

	return true
}

func (s *Strategy) GetPriceLevelByPrice(prc float64) (*PriceLevel, error) {
	if len(s.PriceLevels.Bands) < 1 {
		return nil, fmt.Errorf("Strategy.GetPriceLevelByPrice: PriceLevels must have at least 2 levels")
	}

	for i := 0; i < len(s.PriceLevels.Bands)-1; i++ {
		if prc >= s.PriceLevels.Bands[i].Price && prc < s.PriceLevels.Bands[i+1].Price {
			if s.Direction == Up {
				return s.PriceLevels.Bands[i], nil
			} else if s.Direction == Down {
				return s.PriceLevels.Bands[i+1], nil
			}
		}
	}

	return nil, fmt.Errorf("Strategy.GetPriceLevelByPrice: price levels not found for price = %v, with direction = %v: %w", prc, s.Direction, PriceOutsideLimitsErr)
}

func (s *Strategy) GetPriceLevelByIndex(index int) (*PriceLevel, error) {
	return s.PriceLevels.GetByIndex(index)
}

func (s *Strategy) GetTradesByPriceLevel(openTradesOnly bool) []*TradeLevels {
	var priceLevelTrades []*TradeLevels

	for index, level := range s.PriceLevels.Bands {
		var trades []*TradeDTO

		if openTradesOnly {
			for _, tr := range *level.Trades.OpenTrades() {
				trades = append(trades, tr.ConvertToTradeDTO())
			}
		} else {
			for _, tr := range *level.Trades {
				trades = append(trades, tr.ConvertToTradeDTO())
			}
		}

		priceLevelTrades = append(priceLevelTrades, &TradeLevels{
			PriceLevelIndex: index,
			Trades:          trades,
		})
	}

	return priceLevelTrades
}

func (s *Strategy) GetOpenTrade() *Trades {
	s.getTradesMutex.Lock()
	defer s.getTradesMutex.Unlock()

	trades := Trades{}

	for _, level := range s.PriceLevels.Bands {
		for _, tr := range *level.Trades.OpenTrades() {
			trades = append(trades, tr)
		}
	}

	return &trades
}

func (s *Strategy) GetTrades() *Trades {
	s.getTradesMutex.Lock()
	defer s.getTradesMutex.Unlock()

	trades := Trades{}

	for _, level := range s.PriceLevels.Bands {
		for _, tr := range *level.Trades {
			trades = append(trades, tr)
		}
	}

	return &trades
}

func (s Strategy) String() string {
	return s.Name
}

func (s *Strategy) NewCloseTrades(id uuid.UUID, timeframe *int, timestamp time.Time, requestedPrice float64, priceLevelIndex int, percent float64) (*Trade, []*PartialCloseItemRequest, error) {
	if percent < 0 || percent > 1 {
		return nil, nil, InvalidClosePercentErr
	}

	priceLevel, err := s.GetPriceLevelByIndex(priceLevelIndex)
	if err != nil {
		return nil, nil, fmt.Errorf("Strategy.NewOpenTrade: failed to get price level by index: %w", err)
	}

	openTrades := *priceLevel.Trades.OpenTrades()
	_, openTradesVol, _ := openTrades.GetTradeStatsItems()
	startingVolumeToClose := math.Abs(float64(openTradesVol)) * percent
	volumeToClose := startingVolumeToClose

	trades := make([]*Trade, 0)
	for _, tr := range openTrades {
		remainingVol := tr.RemainingOpenVolume()
		if math.Abs(remainingVol) > SmallRoundingError {
			trades = append(trades, tr)
			closeVol := math.Min(volumeToClose, math.Abs(remainingVol))
			volumeToClose -= closeVol
		}

		if volumeToClose == 0 {
			break
		}
	}

	closeVol := startingVolumeToClose - volumeToClose

	return NewCloseTrade(id, trades, timeframe, timestamp, requestedPrice, closeVol, priceLevel)
}

func (s *Strategy) NewCloseTrade(id uuid.UUID, timeframe *int, timestamp time.Time, requestedPrice float64, percent float64, openTrade *Trade) (*Trade, []*PartialCloseItemRequest, error) {
	if percent < 0 || percent > 1 {
		return nil, nil, InvalidClosePercentErr
	}

	closeVol := openTrade.RemainingOpenVolume() * percent
	return NewCloseTrade(id, Trades{openTrade}, timeframe, timestamp, requestedPrice, closeVol, openTrade.PriceLevel)
}

func (s *Strategy) calculateTradeVolume(priceLevel *PriceLevel, requestedPrice float64) (float64, error) {
	maxLoss := s.Balance * priceLevel.AllocationPercent
	_, _, realizedPL := s.GetTrades().GetTradeStatsItems()
	currentRisk := priceLevel.Trades.CurrentRisk(priceLevel.StopLoss)
	tradesRemaining, _ := priceLevel.NewTradesRemaining()

	var remainingRisk float64
	if float64(tradesRemaining) <= 0 {
		remainingRisk = 0.0
	} else {
		remainingRisk = (maxLoss + float64(realizedPL) - currentRisk) / float64(tradesRemaining)
	}

	if remainingRisk <= 0 {
		return 0.0, fmt.Errorf("Strategy.NewOpenTrade: remainingRisk = %v: %w", remainingRisk, NoRemainingRiskAvailableErr)
	}

	requestedVolume := remainingRisk / math.Abs(requestedPrice-priceLevel.StopLoss)
	return requestedVolume, nil
}

func (s *Strategy) NewOpenTrade(id uuid.UUID, timeframe *int, timestamp time.Time, requestedPrice float64) (*Trade, []*PartialCloseItemRequest, error) {
	priceLevel, err := s.GetPriceLevelByPrice(requestedPrice)
	if err != nil {
		return nil, nil, fmt.Errorf("Strategy.NewOpenTrade: failed to get price level by index: %w", err)
	}

	requestedVolume, err := s.calculateTradeVolume(priceLevel, requestedPrice)
	if err != nil {
		return nil, nil, fmt.Errorf("Strategy.NewOpenTrade: failed to calculate trade volume: %w", err)
	}

	return NewOpenTrade(id, s.GetTradeType(false), s.Symbol, timeframe, timestamp, requestedPrice, requestedVolume, priceLevel.StopLoss, priceLevel)
}

func (s *Strategy) TradesRemaining(price float64) (int, TradeType) {
	_, lvl := s.findPriceLevel(price)
	if lvl != nil {
		return lvl.NewTradesRemaining()
	}
	return 0, TradeTypeBuy
}

func (s *Strategy) findPriceLevel(price float64) (int, *PriceLevel) {
	for i, priceLevel := range s.PriceLevels.Bands[:len(s.PriceLevels.Bands)-1] {
		if price >= s.PriceLevels.Bands[i].Price && price < s.PriceLevels.Bands[i+1].Price {
			if s.Direction == Up {
				return i, priceLevel
			} else if s.Direction == Down {
				return i + 1, s.PriceLevels.Bands[i+1]
			}
		}
	}

	return 0, nil
}

func (s *Strategy) isConditionUnique(signal *SignalV2) bool {
	for _, cond := range s.EntryConditions {
		if cond.EntrySignal.Name == signal.Name {
			return false
		}
	}

	return true
}

func (s *Strategy) RemoveCondition(signal SignalV2) error {
	for i, cond := range s.EntryConditions {
		if cond.EntrySignal.Name == signal.Name {
			s.EntryConditions = append(s.EntryConditions[:i], s.EntryConditions[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("Strategy.RemoveCondition: could not find signal %v", signal)
}

func (s *Strategy) AddExitCondition(name string, levelIndex int, signals []*ExitSignal, resetSignals []*SignalV2, constraints SignalConstraints, closePercent ClosePercent, maxTriggerCount *int) error {
	condition, err := NewExitCondition(name, levelIndex, signals, resetSignals, constraints, closePercent, maxTriggerCount)
	if err != nil {
		return fmt.Errorf("Strategy.AddExitCondition: failed to create new exit condition: %w", err)
	}

	s.ExitConditions = append(s.ExitConditions, condition)

	return nil
}

func (s *Strategy) AddEntryCondition(entrySignal *SignalV2, exitSignal *SignalV2) error {
	if !s.isConditionUnique(entrySignal) {
		return fmt.Errorf("signal %v already exists", entrySignal)
	}

	s.EntryConditions = append(s.EntryConditions, &EntryCondition{
		EntrySignal: entrySignal,
		ResetSignal: exitSignal,
	})

	return nil
}

func (s *Strategy) GetTradeType(isClose bool) TradeType {
	switch s.Direction {
	case Up:
		if isClose {
			return TradeTypeSell
		} else {
			return TradeTypeBuy
		}
	case Down:
		if isClose {
			return TradeTypeBuy
		} else {
			return TradeTypeSell
		}
	default:
		return TradeTypeNone
	}
}

func (s *Strategy) AutoExecuteTrade(trade *Trade) (*ExecuteOpenTradeResult, error) {
	return s.ExecuteOpenTradeRequest(trade, trade.RequestedPrice, trade.RequestedVolume)
}

func (s *Strategy) ExecuteOpenTradeRequest(trade *Trade, price float64, volume float64) (*ExecuteOpenTradeResult, error) {
	s.executeOpenTradeRequestMutex.Lock()
	defer s.executeOpenTradeRequestMutex.Unlock()

	if err := trade.Validate(nil); err != nil {
		return nil, fmt.Errorf("ExecuteTradeRequest failed to Validate trade: %w", err)
	}

	if err := s.CanPlaceTrade(trade, false); err != nil {
		return nil, fmt.Errorf("ExecuteTradeRequest cannot place trade: %w", err)
	}

	// todo: consider scenario where slippage causes a trade to be executed in a different price level
	var reqPrice float64
	if trade.Type == TradeTypeClose {
		if trade.Offsets == nil || len(trade.Offsets) == 0 {
			return nil, fmt.Errorf("ExecuteOpenTradeRequest: closing trade does not have an offset trade")
		}

		reqPrice = trade.Offsets[0].RequestedPrice
	} else {
		reqPrice = trade.RequestedPrice
	}

	priceLevelIndex, priceLevel := s.findPriceLevel(reqPrice)
	if priceLevel == nil {
		return nil, fmt.Errorf("ExecuteTradeRequest failed to findPriceLevel at %.2f", trade.RequestedPrice)
	}

	if err := priceLevel.Add(trade, price, volume); err != nil {
		return nil, fmt.Errorf("ExecuteOpenTradeRequest: failed to add trade to price level %v: %w", priceLevelIndex, err)
	}

	return &ExecuteOpenTradeResult{
		PriceLevelIndex: priceLevelIndex,
		Trade:           trade,
	}, nil
}

func (s *Strategy) CanPlaceTrade(trade *Trade, isClose bool) error {
	if trade.Type == TradeTypeClose {
		return nil
	}

	_, priceLevel := s.findPriceLevel(trade.RequestedPrice)
	if priceLevel == nil {
		return PriceOutsideLimitsErr
	}

	if priceLevel.MaxNoOfTrades <= 0 {
		return MaxTradesPerPriceLevelErr
	}

	tradesRemaining, side := priceLevel.NewTradesRemaining()

	tradeType := s.GetTradeType(isClose)

	if tradeType == TradeTypeBuy {
		if side == TradeTypeBuy && tradesRemaining <= 0 {
			return MaxTradesPerPriceLevelErr
		}
	} else if tradeType == TradeTypeSell {
		if side == TradeTypeSell && tradesRemaining <= 0 {
			return MaxTradesPerPriceLevelErr
		}
	}

	_, _, realizedPL := s.GetTrades().GetTradeStatsItems()

	maxPriceLevelLoss := s.Balance * priceLevel.AllocationPercent
	maxTradeLoss := maxPriceLevelLoss / float64(priceLevel.MaxNoOfTrades)

	if maxTradeLoss-float64(realizedPL) > maxPriceLevelLoss {
		return MaxLossPriceBandErr
	}

	return nil
}

func NewStrategy(name string, symbol string, direction Direction, balance float64, priceLevelInput []*PriceLevel, account *Account) (*Strategy, error) {
	if balance <= 0 {
		return nil, BalanceGreaterThanZeroErr
	}

	strategy := &Strategy{
		Name:            name,
		Symbol:          symbol,
		Direction:       direction,
		EntryConditions: make([]*EntryCondition, 0),
		Balance:         balance,
		Account:         account,
	}

	priceLevels, err := NewPriceLevels(priceLevelInput, direction, strategy)
	if err != nil {
		return nil, fmt.Errorf("NewStrategy: failed to create price levels: %w", err)
	}

	strategy.PriceLevels = priceLevels

	if err = strategy.Validate(); err != nil {
		return nil, fmt.Errorf("NewStrategy: validation failed: %w", err)
	}

	return strategy, nil
}
