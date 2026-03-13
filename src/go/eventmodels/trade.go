package eventmodels

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
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
	FloatingPL float64 `json:"floatingPL"`
	RealizedPL float64 `json:"realizedPL"`
	Volume     Volume  `json:"volume"`
	Vwap       Vwap    `json:"vwap"`
}

type TradeParameters struct {
	PriceLevel *PriceLevel
	MaxLoss    float64
}

type PartialCloseItemRequest struct {
	Trade            *Trade
	PartialCloseItem *PartialCloseItem
}

type PartialCloseItems []*PartialCloseItem

func (it *PartialCloseItems) Contains(trade *Trade) bool {
	for _, item := range *it {
		if item.ClosedBy == trade {
			return true
		}
	}

	return false
}

func (it *PartialCloseItems) Add(item *PartialCloseItem) {
	*it = append(*it, item)
}

func (it *PartialCloseItems) Volume() float64 {
	sum := 0.0

	if it != nil {
		for _, item := range *it {
			sum += item.ExecutedVolume
		}
	}

	return sum
}

type PartialCloseItem struct {
	ClosedBy       *Trade  `json:"closedBy"`
	ExecutedVolume float64 `json:"volume"`
	ExecutedPrice  float64 `json:"price"`
}

type Trade struct {
	ID              uuid.UUID          `json:"id"`
	Type            TradeType          `json:"type"`
	Timeframe       *int               `json:"timeframe"`
	Symbol          string             `json:"symbol"`
	Timestamp       time.Time          `json:"timestamp"`
	RequestedVolume float64            `json:"requestedVolume"`
	ExecutedVolume  float64            `json:"executedVolume"`
	ExecutedPrice   float64            `json:"executedPrice"`
	RequestedPrice  float64            `json:"requestedPrice"`
	StopLoss        float64            `json:"stopLoss"`
	Offsets         []*Trade           `json:"offsets"`
	PartialCloses   *PartialCloseItems `json:"-"`
	PriceLevel      *PriceLevel        `json:"-"`
}

func (tr *Trade) ConvertToTradeDTO() *TradeDTO {
	var pl *float64
	if tr.PartialCloses != nil && len(*tr.PartialCloses) > 0 {
		var realizedPL float64
		for _, partialClose := range *tr.PartialCloses {
			if partialClose.ExecutedVolume < 0 {
				realizedPL += (partialClose.ExecutedPrice - tr.ExecutedPrice) * math.Abs(partialClose.ExecutedVolume)
			} else if partialClose.ExecutedVolume > 0 {
				realizedPL += (tr.ExecutedPrice - partialClose.ExecutedPrice) * partialClose.ExecutedVolume
			}
		}

		pl = &realizedPL
	}

	return &TradeDTO{
		ID:              tr.ID,
		Type:            tr.Type,
		Timeframe:       tr.Timeframe,
		Symbol:          tr.Symbol,
		Timestamp:       tr.Timestamp,
		RequestedVolume: tr.RequestedVolume,
		ExecutedVolume:  tr.ExecutedVolume,
		RemainingVolume: tr.RemainingOpenVolume(),
		ExecutedPrice:   tr.ExecutedPrice,
		RequestedPrice:  tr.RequestedPrice,
		StopLoss:        tr.StopLoss,
		ProfitLoss:      pl,
	}
}

type TradeDTO struct {
	ID              uuid.UUID `json:"id"`
	Type            TradeType `json:"type"`
	Timeframe       *int      `json:"timeframe"`
	Symbol          string    `json:"symbol"`
	Timestamp       time.Time `json:"timestamp"`
	RequestedVolume float64   `json:"requestedVolume"`
	ExecutedVolume  float64   `json:"executedVolume"`
	RemainingVolume float64   `json:"remainingVolume"`
	ExecutedPrice   float64   `json:"executedPrice"`
	RequestedPrice  float64   `json:"requestedPrice"`
	StopLoss        float64   `json:"stopLoss"`
	ProfitLoss      *float64  `json:"profitLoss"`
}

type ClosePercent float64

func (p ClosePercent) Validate() error {
	if p <= 0 || p > 1 {
		return InvalidClosePercentErr
	}

	return nil
}

func (tr *Trade) RealizedPL() float64 {
	realizedPL := 0.0

	if tr.Type == TradeTypeClose {
		for _, offset := range tr.Offsets {
			for _, partialCloseItem := range *offset.PartialCloses {
				if partialCloseItem.ClosedBy == tr {
					if partialCloseItem.ExecutedVolume < 0 {
						realizedPL = (partialCloseItem.ExecutedPrice - offset.ExecutedPrice) * math.Abs(partialCloseItem.ExecutedVolume)
					} else {
						realizedPL = (offset.ExecutedPrice - partialCloseItem.ExecutedPrice) * partialCloseItem.ExecutedVolume
					}
				}
			}
		}
	} else {
		if tr.PartialCloses != nil {
			for _, partialClose := range *tr.PartialCloses {
				if tr.ExecutedPrice <= 0 {
					continue
				}

				if tr.Type == TradeTypeBuy {
					realizedPL += (partialClose.ExecutedPrice - tr.ExecutedPrice) * math.Abs(partialClose.ExecutedVolume)
				} else if tr.Type == TradeTypeSell {
					realizedPL += (tr.ExecutedPrice - partialClose.ExecutedPrice) * partialClose.ExecutedVolume
				}
			}
		}
	}

	return realizedPL
}

func (tr *Trade) RemainingOpenVolume() float64 {
	return tr.ExecutedVolume + tr.ClosedVolume() // tr.ExecutedVolume and tr.ClosedVolume() are opposite signs
}

func (tr *Trade) ClosedVolume() float64 {
	return tr.PartialCloses.Volume()
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

func (tr *Trade) String() string {
	volumeStr := strconv.FormatFloat(tr.RequestedVolume, 'f', 8, 64)

	realizedPL := tr.RealizedPL()

	return fmt.Sprintf("%s %s @%.2f, realizedPL %.2f", volumeStr, tr.Symbol, tr.ExecutedPrice, realizedPL)
}

func (tr *Trade) Validate(partialCloseItems []*PartialCloseItemRequest) error {
	if tr.ID == uuid.Nil {
		return NoTradeIDErr
	}

	if tr.Symbol == "" {
		return SymbolNotSetErr
	}

	if tr.Timeframe != nil && *tr.Timeframe <= 0 {
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
			totalOffsetVolume += tr.Offsets[i].RemainingOpenVolume()

			if math.Abs(totalOffsetVolume) >= math.Abs(tr.RequestedVolume) && i != len(tr.Offsets)-1 {
				return OffsetTradesVolumeExceedsClosingTradeVolumeErr
			}
		}

		if math.Abs(tr.RequestedVolume) > math.Abs(totalOffsetVolume)+SmallRoundingError {
			return ErrDuplicateCloseTrade
		}
	}

	// validate partial close items
	if partialCloseItems != nil {
		if tr.Type != TradeTypeClose {
			return fmt.Errorf("found %v. Only TradeTypeClose should contain partialCloseItems", tr.Type)
		}

		totalPartialCloseVolume := 0.0
		for i := 0; i < len(partialCloseItems); i += 1 {
			totalPartialCloseVolume += partialCloseItems[i].PartialCloseItem.ExecutedVolume
		}

		if math.Abs(totalPartialCloseVolume-tr.RequestedVolume) > SmallRoundingError {
			return fmt.Errorf("sum of partial close items should equal the closing trade's requested volume")
		}
	}

	return nil
}

// PreparePartialCloseItems creates that will be added to offset trades, without actually adding
// them, in case a failure happens further down the stack
func (tr *Trade) PreparePartialCloseItems(executedPrice float64, executedVolume float64) ([]*PartialCloseItemRequest, error) {
	if tr.Type != TradeTypeClose {
		return nil, nil
	}

	// record partial closes to offset offsetTrades
	offsetTrades := tr.Offsets
	totalClosedVolume := 0.0
	partialCloseItems := make([]*PartialCloseItemRequest, 0)
	for i := 0; i < len(offsetTrades)-1; i++ {
		offsetTradeRemainingVolume := offsetTrades[i].RemainingOpenVolume()
		if math.Abs(offsetTradeRemainingVolume) < SmallRoundingError {
			return nil, fmt.Errorf("ModifyOffsetTradesToAddPartialCloseItem: trade %v has no remaining volume to serve as an offset trade", offsetTrades[i])
		}

		partialCloseItems = append(partialCloseItems, &PartialCloseItemRequest{
			Trade: offsetTrades[i],
			PartialCloseItem: &PartialCloseItem{
				ClosedBy:       tr,
				ExecutedVolume: offsetTradeRemainingVolume * -1,
				ExecutedPrice:  executedPrice,
			},
		})

		totalClosedVolume += offsetTradeRemainingVolume
	}

	remainingVolumeToClose := executedVolume + totalClosedVolume // executedVolume and totalClosedVolume are opposite signs

	partialCloseItems = append(partialCloseItems, &PartialCloseItemRequest{
		Trade: offsetTrades[len(offsetTrades)-1],
		PartialCloseItem: &PartialCloseItem{
			ClosedBy:       tr,
			ExecutedVolume: remainingVolumeToClose,
			ExecutedPrice:  executedPrice,
		},
	})

	return partialCloseItems, nil
}

func (tr *Trade) IsStopLossTriggered(tick Tick) (*CloseTradeRequestV2, error) {
	switch tr.Type {
	case TradeTypeBuy:
		if tick.Price <= tr.StopLoss {
			return &CloseTradeRequestV2{
				Trade:     tr,
				Timeframe: nil,
				Percent:   1.0,
				Reason:    "sl",
			}, nil
		}
	case TradeTypeSell:
		if tick.Price >= tr.StopLoss {
			return &CloseTradeRequestV2{
				Trade:     tr,
				Timeframe: nil,
				Percent:   1.0,
				Reason:    "sl",
			}, nil
		}
	}

	return nil, nil
}

// Execute sets the actual price that the trade was executed at when sending the trade to the market
func (tr *Trade) Execute(executedPrice float64, executedVolume float64) error {
	partialCloseItems, err := tr.PreparePartialCloseItems(executedPrice, executedVolume)
	if err != nil {
		return fmt.Errorf("Trade.Execute: failed to prepare partial close items: %w", err)
	}

	if err = tr.Validate(partialCloseItems); err != nil {
		return fmt.Errorf("Trade.Execute: trade is not valid: %w", err)
	}

	for _, it := range partialCloseItems {
		it.Trade.PartialCloses.Add(it.PartialCloseItem)
	}

	tr.ExecutedPrice = executedPrice
	tr.ExecutedVolume = executedVolume
	return nil
}

// AutoExecute sets the executed price to the requested price
func (tr *Trade) AutoExecute() {
	tr.Execute(tr.RequestedPrice, tr.RequestedVolume)
}

func newTrade(id uuid.UUID, tradeType TradeType, symbol string, timeframe *int, timestamp time.Time, requestedPrice float64, requestedVolume float64, stopLoss float64, offsets []*Trade, priceLevel *PriceLevel) (*Trade, []*PartialCloseItemRequest, error) {
	var vol float64
	var volSign float64 // placeholder for volume: +1 if buy, else -1

	switch tradeType {
	case TradeTypeBuy:
		vol = math.Abs(requestedVolume)
		volSign = 1.0
	case TradeTypeSell:
		vol = -math.Abs(requestedVolume)
		volSign = -1.0
	case TradeTypeClose:
		if offsets == nil || len(offsets) == 0 {
			return nil, nil, fmt.Errorf("newTrade: offset trade not set")
		}

		switch offsets[0].Type {
		case TradeTypeBuy:
			vol = -math.Abs(requestedVolume)
			volSign = -1.0
		case TradeTypeSell:
			vol = math.Abs(requestedVolume)
			volSign = 1.0
		default:
			return nil, nil, fmt.Errorf("newTrade: unknown trade type %v for offset trade", tradeType)
		}
	default:
		return nil, nil, fmt.Errorf("newTrade: unknown trade type %v", tradeType)
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
		PartialCloses:   &PartialCloseItems{},
		PriceLevel:      priceLevel,
	}

	// add partial closes
	var partialCloseItemRequests []*PartialCloseItemRequest
	if len(offsets) > 0 {
		absVol := math.Abs(vol)
		for _, offset := range offsets {
			reduceVolumeBy := math.Min(absVol, math.Abs(offset.ExecutedVolume))
			partialCloseItemRequests = append(partialCloseItemRequests, &PartialCloseItemRequest{
				Trade: offset,
				PartialCloseItem: &PartialCloseItem{
					ClosedBy:       trade,
					ExecutedVolume: reduceVolumeBy * volSign,
					ExecutedPrice:  0.0,
				},
			})

			absVol -= reduceVolumeBy
		}

		if absVol != 0 {
			return nil, nil, fmt.Errorf("remaining absVol(%v) != 0: %w", absVol, ErrDuplicateCloseTrade)
		}
	}

	// I EITHER HAVE TO IGNORE THE DUPLICATE OR PREVENT IT FROM HAPPENING

	if err := trade.Validate(nil); err != nil {
		return nil, nil, fmt.Errorf("newTrade: failed to open new trade: %w", err)
	}

	return trade, partialCloseItemRequests, nil
}

func NewOpenTrade(id uuid.UUID, tradeType TradeType, symbol string, timeframe *int, timestamp time.Time, requestedPrice float64, requestedVolume float64, stopLoss float64, priceLevel *PriceLevel) (*Trade, []*PartialCloseItemRequest, error) {
	return newTrade(id, tradeType, symbol, timeframe, timestamp, requestedPrice, requestedVolume, stopLoss, nil, priceLevel)
}

func NewCloseTrade(id uuid.UUID, trades []*Trade, timeframe *int, timestamp time.Time, requestedPrice float64, requestedVolume float64, priceLevel *PriceLevel) (*Trade, []*PartialCloseItemRequest, error) {
	if len(trades) == 0 {
		return nil, nil, fmt.Errorf("NewTradeClose: %w", NoOffsettingTradeErr)
	}

	symbol := trades[0].Symbol
	for _, tr := range trades[1:] {
		if tr.Symbol != symbol {
			return nil, nil, fmt.Errorf("NewTradeClose: all trades must have the same symbol. Found %v and %v", tr.Symbol, symbol)
		}
	}

	for _, offset := range trades {
		if math.Abs(offset.ExecutedVolume-offset.ClosedVolume()) < SmallRoundingError {
			return nil, nil, fmt.Errorf("NewCloseTrade: trade %v cannot be used as on offset: it is already closed", offset)
		}
	}

	return newTrade(id, TradeTypeClose, symbol, timeframe, timestamp, requestedPrice, requestedVolume, 0, trades, priceLevel)
}
