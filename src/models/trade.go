package models

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"strconv"
	"time"
)

type TradeType int

const (
	TradeTypeBuy TradeType = iota
	TradeTypeSell
	TradeTypeClose
	TradeTypeNone
)

func (t TradeType) String() string {
	switch t {
	case TradeTypeBuy:
		return "buy"
	case TradeTypeSell:
		return "sell"
	case TradeTypeClose:
		return "close"
	default:
		return "unknown"
	}
}

type TradeStats struct {
	Floating float64
	Realized float64
	Volume   Volume
}

type TradeParameters struct {
	PriceLevel *PriceLevel
	MaxLoss    float64
}

type Trade struct {
	ID              uuid.UUID `json:"id"`
	Type            TradeType `json:"type"`
	Timeframe       int       `json:"timeframe"`
	Symbol          string    `json:"symbol"`
	Timestamp       time.Time `json:"timestamp"`
	RequestedVolume float64   `json:"requestedVolume"`
	ExecutedVolume  float64   `json:"executedVolume"`
	ExecutedPrice   float64   `json:"executedPrice"`
	RequestedPrice  float64   `json:"requestedPrice"`
	StopLoss        float64   `json:"stopLoss"`
	Offsets         []*Trade  `json:"offsets"`
}

type ClosePercent float64

func (p ClosePercent) Validate() error {
	if p <= 0 || p > 1 {
		return InvalidClosePercentErr
	}

	return nil
}

func (tr *Trade) Side() TradeType {
	if tr.RequestedVolume > 0 {
		return TradeTypeBuy
	}

	if tr.RequestedVolume < 0 {
		return TradeTypeSell
	}

	return TradeTypeNone
}

func (tr Trade) String() string {
	volumeStr := strconv.FormatFloat(tr.RequestedVolume, 'f', -1, 64)
	return fmt.Sprintf("%s %s @%.2f", volumeStr, tr.Symbol, tr.ExecutedPrice)
}

func (tr *Trade) Validate() error {
	if tr.ID == uuid.Nil {
		return NoTradeIDErr
	}

	if tr.Symbol == "" {
		return SymbolNotSetErr
	}

	if tr.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	if tr.Type != TradeTypeBuy && tr.Type != TradeTypeSell && tr.Type != TradeTypeClose {
		return UnknownTradeTypeErr
	}

	if tr.Timestamp.IsZero() {
		return NoTimestampErr
	}

	if tr.RequestedPrice <= 0 {
		return InvalidRequestedPriceErr
	}

	if tr.StopLoss < 0 {
		return NegativeStopLossErr
	}

	if tr.StopLoss > 0 {
		if tr.Type == TradeTypeBuy && tr.StopLoss >= tr.RequestedPrice {
			return fmt.Errorf("stop loss must be less than requested price for buy orders: %w", InvalidStopLossErr)
		} else if tr.Type == TradeTypeSell && tr.StopLoss <= tr.RequestedPrice {
			return fmt.Errorf("stop loss must be greater than requested price for sell orders: %w", InvalidStopLossErr)
		}
	}

	if tr.Type != TradeTypeClose && tr.StopLoss == 0 {
		return NoStopLossErr
	}

	if tr.RequestedVolume == 0 {
		return TradeVolumeIsZeroErr
	}

	if tr.RequestedVolume == math.NaN() {
		return fmt.Errorf("requested volume cannot be NaN")
	}

	if math.IsInf(tr.RequestedVolume, 0) {
		return fmt.Errorf("requested volume cannot be +/- Inf")
	}

	if tr.ExecutedVolume == math.NaN() {
		return fmt.Errorf("executed volume cannot be NaN")
	}

	if math.IsInf(tr.ExecutedVolume, 0) {
		return fmt.Errorf("executed volume cannot be +/- Inf")
	}

	if tr.RequestedPrice == math.NaN() {
		return fmt.Errorf("requested price cannot be NaN")
	}

	if math.IsInf(tr.RequestedPrice, 0) {
		return fmt.Errorf("requested price cannot be +/- Inf")
	}

	if tr.ExecutedPrice == math.NaN() {
		return fmt.Errorf("executed price cannot be NaN")
	}

	if math.IsInf(tr.ExecutedPrice, 0) {
		return fmt.Errorf("executed price cannot be +/- Inf")
	}

	if tr.StopLoss == math.NaN() {
		return fmt.Errorf("stop loss cannot be NaN")
	}

	if math.IsInf(tr.StopLoss, 0) {
		return fmt.Errorf("stop loss cannot be +/- Inf")
	}

	if len(tr.Offsets) > 0 {
		totalOffsetVolume := 0.0
		for i := 0; i < len(tr.Offsets); i += 1 {
			totalOffsetVolume += tr.Offsets[i].ExecutedVolume

			if math.Abs(totalOffsetVolume) >= math.Abs(tr.RequestedVolume) && i != len(tr.Offsets)-1 {
				return OffsetTradesVolumeExceedsClosingTradeVolumeErr
			}
		}

		if math.Abs(tr.RequestedVolume) > math.Abs(totalOffsetVolume) {
			return InvalidClosingTradeVolumeErr
		}
	}

	return nil
}

// Execute sets the actual price that the trade was executed at when sending the trade to the market
func (tr *Trade) Execute(executedPrice float64, executedVolume float64) {
	tr.ExecutedPrice = executedPrice
	tr.ExecutedVolume = executedVolume
}

// AutoExecute sets the executed price to the requested price
func (tr *Trade) AutoExecute() {
	tr.ExecutedPrice = tr.RequestedPrice
	tr.ExecutedVolume = tr.RequestedVolume
}

func newTrade(id uuid.UUID, tradeType TradeType, symbol string, timeframe int, timestamp time.Time, requestedPrice float64, requestedVolume float64, stopLoss float64, offsets []*Trade) (*Trade, error) {
	var vol float64

	switch tradeType {
	case TradeTypeBuy:
		vol = math.Abs(requestedVolume)
	case TradeTypeSell:
		vol = -math.Abs(requestedVolume)
	case TradeTypeClose:
		if offsets == nil || len(offsets) == 0 {
			return nil, fmt.Errorf("newTrade: offset trade not set")
		}

		switch offsets[0].Type {
		case TradeTypeBuy:
			vol = -math.Abs(requestedVolume)
		case TradeTypeSell:
			vol = math.Abs(requestedVolume)
		default:
			return nil, fmt.Errorf("newTrade: unknown trade type %v for offset trade", tradeType)
		}
	default:
		return nil, fmt.Errorf("newTrade: unknown trade type %v", tradeType)
	}

	trade := &Trade{
		ID:              id,
		Symbol:          symbol,
		Timeframe:       timeframe,
		Type:            tradeType,
		Timestamp:       timestamp,
		RequestedPrice:  requestedPrice,
		RequestedVolume: vol,
		StopLoss:        stopLoss,
		Offsets:         offsets,
	}

	if err := trade.Validate(); err != nil {
		return nil, fmt.Errorf("newTrade: failed to open new trade: %w", err)
	}

	return trade, nil
}

func NewOpenTrade(id uuid.UUID, tradeType TradeType, symbol string, timeframe int, timestamp time.Time, requestedPrice float64, requestedVolume float64, stopLoss float64) (*Trade, error) {
	return newTrade(id, tradeType, symbol, timeframe, timestamp, requestedPrice, requestedVolume, stopLoss, nil)
}

func NewCloseTrade(id uuid.UUID, trades []*Trade, timeframe int, timestamp time.Time, requestedPrice float64, requestedVolume float64) (*Trade, error) {
	if len(trades) == 0 {
		return nil, fmt.Errorf("NewTradeClose: %w", NoOffsettingTradeErr)
	}

	symbol := trades[0].Symbol
	for _, tr := range trades[:1] {
		if tr.Symbol != symbol {
			return nil, fmt.Errorf("NewTradeClose: all trades must have the same symbol. Found %v and %v", tr.Symbol, symbol)
		}
	}

	return newTrade(id, TradeTypeClose, symbol, timeframe, timestamp, requestedPrice, requestedVolume, 0, trades)
}
