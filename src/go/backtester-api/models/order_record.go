package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

type Attributes map[string]string

func (a *Attributes) Add(key, value string) {
	if *a == nil {
		*a = make(map[string]string)
	}
	(*a)[key] = value
}

func (a *Attributes) Scan(value interface{}) error {
	if value == nil {
		*a = make(map[string]string)
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, a)
	case string:
		return json.Unmarshal([]byte(v), a)
	default:
		return fmt.Errorf("unsupported type for Attributes: %T", value)
	}
}

func (a Attributes) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}

	return json.Marshal(a)
}

type OrderRecord struct {
	gorm.Model
	PlaygroundID     uuid.UUID              `gorm:"column:playground_id;type:uuid;not null;index:idx_playground_order;index:idx_playground_client_request_id,priority:1" copier:"must,nopanic"`
	ClientRequestID  *string                `gorm:"column:client_request_id;type:text;index:idx_playground_client_request_id,priority:2" copier:"must,nopanic"`
	LiveAccountType  LiveAccountType        `gorm:"column:account_type;type:text;not null" copier:"must,nopanic"`
	ExternalOrderID  *uint                  `gorm:"column:external_id;index:idx_external_order_id" copier:"must,nopanic"`
	Class            OrderRecordClass       `gorm:"column:class;type:text;not null" copier:"must,nopanic"`
	Symbol           string                 `gorm:"column:symbol;type:text;not null" copier:"must,nopanic"`
	Side             TradierOrderSide       `gorm:"column:side;type:text;not null" copier:"must,nopanic"`
	AbsoluteQuantity float64                `gorm:"column:quantity;type:numeric;not null" copier:"must,nopanic"`
	OrderType        OrderRecordType        `gorm:"column:order_type;type:text;not null" copier:"must,nopanic"`
	Duration         OrderRecordDuration    `gorm:"column:duration;type:text;not null" copier:"must,nopanic"`
	Price            *float64               `gorm:"column:price;type:numeric" copier:"must,nopanic"`
	RequestedPrice   float64                `gorm:"column:requested_price;type:numeric" copier:"must,nopanic"`
	StopPrice        *float64               `gorm:"column:stop_price;type:numeric" copier:"must,nopanic"`
	Status           OrderRecordStatus      `gorm:"column:status;type:text;not null" copier:"must,nopanic"`
	RejectReason     *string                `gorm:"column:reject_reason;type:text" copier:"must,nopanic"`
	Tag              string                 `gorm:"column:tag;type:text" copier:"must,nopanic"`
	Timestamp        time.Time              `gorm:"column:timestamp;type:timestamptz;not null" copier:"must,nopanic"`
	IsAdjustment     bool                   `gorm:"column:is_adjustment" copier:"must,nopanic"`
	IsClose          bool                   `gorm:"-" copier:"must,nopanic"`
	CloseOrderId     *uint                  `gorm:"column:close_order_id" copier:"must,nopanic"`
	IsSystemOrder    bool                   `gorm:"column:is_system_order" copier:"must,nopanic"`
	Closes           []*OrderRecord         `gorm:"many2many:order_closes" copier:"must,nopanic"`
	ClosedBy         []*TradeRecord         `gorm:"many2many:trade_closed_by" copier:"must,nopanic"`
	Reconciles       []*OrderRecord         `gorm:"many2many:order_reconciles" copier:"must,nopanic"`
	PreviousPosition Position               `gorm:"type:json" copier:"must,nopanic"`
	Trades           []*TradeRecord         `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" copier:"must,nopanic"`
	ReconcileTrades  []*TradeRecord         `gorm:"foreignKey:ReconcileOrderID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" copier:"must,nopanic"`
	instrument       eventmodels.Instrument `gorm:"-" copier:"must,nopanic"`
	PreviousBalance  *float64               `gorm:"column:previous_balance;type:numeric" copier:"must,nopanic"`
	Attributes       Attributes             `gorm:"column:attributes;type:jsonb" copier:"must,nopanic"`
}

// tradeMatchesOrder checks if a ClosedBy trade belongs to the given closing order.
// A trade matches if it is directly one of the order's trades (by ID) or if it is
// a partial trade whose parent is one of the order's trades.
func tradeMatchesOrder(tr *TradeRecord, closingOrder *OrderRecord) bool {
	for _, thisTr := range closingOrder.Trades {
		if tr.ID == thisTr.ID {
			return true
		}
		if tr.ParentTradeID != nil && *tr.ParentTradeID == thisTr.ID {
			return true
		}
		if tr.ParentTrade != nil && tr.ParentTrade.ID == thisTr.ID {
			return true
		}
	}
	return false
}

func (o *OrderRecord) CalcRealizedPL() float64 {
	realizedPL := 0.0

	if o.Side == TradierOrderSideBuy || o.Side == TradierOrderSideBuyToOpen {
		vwap := o.GetAvgFillPrice()

		for _, trade := range o.ClosedBy {
			if trade.Quantity > 0 {
				continue
			}

			realizedPL += (trade.Price - vwap) * math.Abs(trade.Quantity)
		}
	} else if o.Side == TradierOrderSideSellShort || o.Side == TradierOrderSideSellToOpen {
		vwap := o.GetAvgFillPrice()

		for _, trade := range o.ClosedBy {
			if trade.Quantity < 0 {
				continue
			}

			realizedPL += (vwap - trade.Price) * trade.Quantity
		}
	} else if o.Side == TradierOrderSideSell || o.Side == TradierOrderSideSellToClose {
		for _, order := range o.Closes {
			vwap := order.GetAvgFillPrice()
			for _, tr := range order.ClosedBy {
				if tradeMatchesOrder(tr, o) {
					realizedPL += (tr.Price - vwap) * math.Abs(tr.Quantity)
				}
			}
		}
	} else if o.Side == TradierOrderSideBuyToCover || o.Side == TradierOrderSideBuyToClose {
		for _, order := range o.Closes {
			vwap := order.GetAvgFillPrice()
			for _, tr := range order.ClosedBy {
				if tradeMatchesOrder(tr, o) {
					realizedPL += (vwap - tr.Price) * math.Abs(tr.Quantity)
				}
			}
		}
	}

	if o.Class == OrderRecordClassOption {
		realizedPL *= 100
	}

	return realizedPL
}

func (o *OrderRecord) GetIsSystemOrder() bool {
	return o.IsSystemOrder
}

func (o *OrderRecord) CreateCloseOrderRequests(positionCache *PositionsCache, timestamp time.Time, requestedPrice float64, requestedQuantity *float64, tag string) ([]*CreateOrderRequest, error) {
	if o.IsClose {
		return nil, fmt.Errorf("CreateCloseOrder: open order %d is not marked as close", o.ID)
	}

	if o.Side != TradierOrderSideBuy && o.Side != TradierOrderSideBuyToOpen && o.Side != TradierOrderSideSellToOpen && o.Side != TradierOrderSideSellShort {
		return nil, fmt.Errorf("CreateCloseOrder: unsupported order side: %s", o.Side)
	}

	openQty, err := o.GetRemainingOpenQuantity()
	if err != nil {
		return nil, fmt.Errorf("CreateCloseOrder: failed to get remaining open quantity: %w", err)
	}

	optionCloseQty := openQty

	var stockOrderRequest *CreateOrderRequest
	var side TradierOrderSide
	switch o.Class {
	case OrderRecordClassEquity:
		if openQty > 0 {
			side = TradierOrderSideSell
		} else if openQty < 0 {
			side = TradierOrderSideBuyToCover
		} else {
			return nil, fmt.Errorf("CreateCloseOrder: open order %d has no remaining open quantity", o.ID)
		}
	case OrderRecordClassOption:
		if openQty > 0 {
			side = TradierOrderSideSellToClose

			// option expired ITM
			if requestedPrice > 0 {
				optionContract, ok := o.GetInstrument().(*eventmodels.OptionContractV3)

				if !ok {
					return nil, fmt.Errorf("CreateCloseOrder: failed to cast instrument to OptionSymbol")
				}

				if optionContract.OptionType == eventmodels.OptionTypeCall {
					var buyQty float64
					if requestedQuantity != nil {
						if *requestedQuantity < 0 {
							return nil, fmt.Errorf("CreateCloseOrder: requested quantity cannot be negative")
						}

						if *requestedQuantity > openQty {
							return nil, fmt.Errorf("CreateCloseOrder: requested quantity cannot be greater than open quantity")
						}

						optionCloseQty = *requestedQuantity
					}

					buyQty = math.Abs(optionCloseQty) * float64(optionContract.ContractSize)

					stockOrderRequest = &CreateOrderRequest{
						Symbol:         string(optionContract.UnderlyingSymbol),
						Class:          OrderRecordClassEquity,
						Quantity:       buyQty,
						Side:           TradierOrderSideBuy,
						OrderType:      Market,
						Duration:       Day,
						RequestedPrice: optionContract.Strike,
						Tag:            fmt.Sprintf("exercise-call-option-%d", o.ID),
						IsAdjustment:   false,
						IsSystemOrder:  true,
					}
				}
			}
		} else if openQty < 0 {
			side = TradierOrderSideBuyToClose

			// option expired ITM
			if requestedPrice > 0 {
				optionContract, ok := o.GetInstrument().(*eventmodels.OptionContractV3)

				if !ok {
					return nil, fmt.Errorf("CreateCloseOrder: failed to cast instrument to OptionSymbol")
				}

				if requestedQuantity != nil {
					if *requestedQuantity < 0 {
						return nil, fmt.Errorf("CreateCloseOrder: requested quantity cannot be negative")
					}

					if *requestedQuantity > math.Abs(openQty) {
						return nil, fmt.Errorf("CreateCloseOrder: requested quantity cannot be greater than (sell) open quantity")
					}

					optionCloseQty = *requestedQuantity
				}

				switch optionContract.OptionType {
				case eventmodels.OptionTypeCall:
					stockOpenQty := 0.0
					currentPosition := positionCache.Get(optionContract.UnderlyingSymbol.GetTicker())
					if currentPosition != nil {
						stockOpenQty = math.Abs(currentPosition.Quantity)
					}

					sellQty := math.Abs(optionCloseQty) * float64(optionContract.ContractSize)

					// exercise call option
					// 1: sell underlying stock if any existing qty to sell
					if stockOpenQty > 0 {
						stockOrderRequest = &CreateOrderRequest{
							Symbol:         string(optionContract.UnderlyingSymbol),
							Class:          OrderRecordClassEquity,
							Quantity:       math.Min(sellQty, stockOpenQty),
							Side:           TradierOrderSideSell,
							OrderType:      Market,
							Duration:       Day,
							RequestedPrice: optionContract.Strike,
							Tag:            fmt.Sprintf("exercise-call-option-%d", o.ID),
							IsAdjustment:   false,
							IsSystemOrder:  true,
						}
					}

					// 2: sell short if any remaining qty to sell
					remainingSellQty := sellQty - stockOpenQty
					if remainingSellQty > 0 {
						stockOrderRequest = &CreateOrderRequest{
							Symbol:         string(optionContract.UnderlyingSymbol),
							Class:          OrderRecordClassEquity,
							Quantity:       remainingSellQty,
							Side:           TradierOrderSideSellShort,
							OrderType:      Market,
							Duration:       Day,
							RequestedPrice: optionContract.Strike,
							Tag:            fmt.Sprintf("exercise-call-option-%d", o.ID),
							IsAdjustment:   false,
							IsSystemOrder:  true,
						}
					}

				case eventmodels.OptionTypePut:
					buyQty := math.Abs(optionCloseQty) * float64(optionContract.ContractSize)

					stockOrderRequest = &CreateOrderRequest{
						Symbol:         string(optionContract.UnderlyingSymbol),
						Class:          OrderRecordClassEquity,
						Quantity:       buyQty,
						Side:           TradierOrderSideBuy,
						OrderType:      Market,
						Duration:       Day,
						RequestedPrice: optionContract.Strike,
						Tag:            fmt.Sprintf("exercise-put-option-%d", o.ID),
						IsAdjustment:   false,
						IsSystemOrder:  true,
					}
				}
			}
		} else {
			return nil, fmt.Errorf("CreateCloseOrder: open order %d has no remaining open quantity", o.ID)
		}
	}

	// Propagate attributes from the open order so that reports can
	// associate auto-close P&L with the original strategy entry.
	var closeAttributes Attributes
	if len(o.Attributes) > 0 {
		closeAttributes = make(Attributes, len(o.Attributes)+1)
		for k, v := range o.Attributes {
			closeAttributes[k] = v
		}
		// Override action so the report can distinguish system closes
		closeAttributes["action"] = tag
	}

	closeOrderRequests := []*CreateOrderRequest{
		{
			Symbol:         o.Symbol,
			Class:          o.Class,
			Quantity:       math.Abs(optionCloseQty),
			Side:           side,
			OrderType:      Market,
			Duration:       Day,
			RequestedPrice: 0.0,
			Tag:            tag,
			CloseOrderId:   &o.ID,
			IsAdjustment:   false,
			IsSystemOrder:  true,
			Attributes:     closeAttributes,
		},
	}

	if stockOrderRequest != nil {
		closeOrderRequests = append(closeOrderRequests, stockOrderRequest)
	}

	return closeOrderRequests, nil
}

func (o *OrderRecord) GetTrades() []*TradeRecord {
	if o.LiveAccountType == LiveAccountTypeReconcilation {
		return o.ReconcileTrades
	}

	return o.Trades
}

func (o *OrderRecord) IsPending() bool {
	if o.Status == OrderRecordStatusPending {
		return true
	}

	for _, o2 := range o.Reconciles {
		if o2.Status == OrderRecordStatusPending {
			return true
		}
	}

	return false
}

func (o *OrderRecord) GetInstrument() eventmodels.Instrument {
	if o.instrument == nil {
		switch o.Class {
		case OrderRecordClassEquity:
			o.instrument = eventmodels.NewStockSymbol(o.Symbol)
		case OrderRecordClassOption:
			o.instrument = eventmodels.OptionSymbol(o.Symbol)
		default:
			panic(fmt.Sprintf("unsupported order record class: %s", o.Class))
		}
	}

	return o.instrument
}

func (o *OrderRecord) Cancel() {
	o.Status = OrderRecordStatusCanceled
}

func (o *OrderRecord) Reject(err error) {
	reason := err.Error()
	o.RejectReason = &reason
	o.Status = OrderRecordStatusRejected
}

func (o *OrderRecord) GetQuantity() float64 {
	if o.Side == TradierOrderSideSell || o.Side == TradierOrderSideSellShort || o.Side == TradierOrderSideSellToOpen || o.Side == TradierOrderSideSellToClose {
		return -o.AbsoluteQuantity
	}

	return o.AbsoluteQuantity
}

func (o *OrderRecord) Rollback(trade *TradeRecord) {
	if o.LiveAccountType == LiveAccountTypeReconcilation {
		for i, t := range o.ReconcileTrades {
			if t.ID == trade.ID {
				o.ReconcileTrades = append(o.ReconcileTrades[:i], o.ReconcileTrades[i+1:]...)
				break
			}
		}
	} else {
		for i, t := range o.Trades {
			if t.ID == trade.ID {
				o.Trades = append(o.Trades[:i], o.Trades[i+1:]...)
				break
			}
		}
	}

	o.Reject(fmt.Errorf("trade rolled back, (qty, prc)=(%.2f, %.2f)", trade.Quantity, trade.Price))
}

func (o *OrderRecord) Fill(trade *TradeRecord) (bool, error) {
	if !o.Status.IsTradingAllowed() {
		return false, ErrTradingNotAllowed
	}

	if o.Class == OrderRecordClassEquity && trade.Price <= 0 {
		return false, fmt.Errorf("trade price must be greater than 0")
	}

	filledQuantity := 0.0
	trades := o.GetTrades()
	for _, t := range trades {
		filledQuantity += t.Quantity
	}

	if trade.Quantity == 0 {
		return false, fmt.Errorf("trade quantity must be non-zero")
	}

	if math.Abs(filledQuantity) == o.AbsoluteQuantity {
		o.Status = OrderRecordStatusFilled
		return true, ErrOrderAlreadyFilled
	}

	if o.Class == OrderRecordClassEquity {
		if math.Abs(trade.Quantity+filledQuantity) > o.AbsoluteQuantity {
			return false, fmt.Errorf("trade quantity (%.2f + %.2f) exceeds order quantity (%.2f)", trade.Quantity, filledQuantity, o.AbsoluteQuantity)
		}
	}

	if o.LiveAccountType == LiveAccountTypeReconcilation {
		o.ReconcileTrades = append(o.ReconcileTrades, trade)
	} else {
		o.Trades = append(o.Trades, trade)
	}

	orderIsFilled := false
	if o.IsFilled() {
		o.Status = OrderRecordStatusFilled
		orderIsFilled = true
	}

	trade.UpdateOrder(o)

	return orderIsFilled, nil
}

func (o *OrderRecord) Hydrate() error {
	if o.instrument == nil {
		if o.Symbol != "" {
			switch o.Class {
			case OrderRecordClassEquity:
				o.instrument = eventmodels.NewStockSymbol(o.Symbol)
			case OrderRecordClassOption:
				o.instrument = eventmodels.OptionSymbol(o.Symbol)
			default:
				return fmt.Errorf("unsupported order record class: %s", o.Class)
			}
		}
	}

	for _, o2 := range o.Reconciles {
		if err := o2.Hydrate(); err != nil {
			return fmt.Errorf("failed to hydrate order record: %w", err)
		}
	}

	for _, o2 := range o.Closes {
		if err := o2.Hydrate(); err != nil {
			return fmt.Errorf("failed to hydrate order record: %w", err)
		}
	}

	return nil
}

func (o *OrderRecord) ResetStatusToPending(dbService IDatabaseService) (commit func() error, dbCommit func() error, msg string) {
	if o.Status != OrderRecordStatusPending {
		commit = func() error {
			o.Status = OrderRecordStatusPending
			o.RejectReason = nil
			return nil
		}

		dbCommit = func() error {
			copy := CopyOrderRecord(o.PlaygroundID, o.ID, o, o.LiveAccountType)
			copy.Status = OrderRecordStatusPending
			copy.RejectReason = nil

			if err := dbService.SaveOrderRecord(copy, nil, false); err != nil {
				return fmt.Errorf("failed to update order record status: %w", err)
			}

			return nil
		}

		msg = fmt.Sprintf("Resetting order %d status from %s to %s", o.ID, o.Status, OrderRecordStatusPending)

		return commit, dbCommit, msg
	}

	return nil, nil, ""
}

func (o *OrderRecord) GetStatus() OrderRecordStatus {
	if !o.Status.IsTradingAllowed() {
		return o.Status
	}

	trades := o.GetTrades()
	if len(trades) == 0 {
		return OrderRecordStatusNew // TODO: change this to pending
	}

	if o.IsFilled() {
		return OrderRecordStatusFilled
	}

	return OrderRecordStatusPartiallyFilled
}

func (o *OrderRecord) GetRemainingOpenQuantity() (float64, error) {
	closedQty := 0.0
	for _, trade := range o.ClosedBy {
		closedQty += trade.Quantity
	}

	if o.Side == TradierOrderSideBuy || o.Side == TradierOrderSideBuyToOpen {
		return math.Max(0, o.GetFilledVolume()+closedQty), nil
	} else if o.Side == TradierOrderSideSellShort || o.Side == TradierOrderSideSellToOpen {
		return math.Min(0, o.GetFilledVolume()+closedQty), nil
	} else {
		return 0, fmt.Errorf("GetRemainingOpenQuantity: unsupported order side")
	}
}

func (o *OrderRecord) IsFilled() bool {
	return o.GetFilledVolume() == o.GetQuantity()
}

func (o *OrderRecord) GetFilledVolume() float64 {
	filledVolume := 0.0
	trades := o.GetTrades()
	for _, trade := range trades {
		filledVolume += trade.Quantity
	}

	return filledVolume
}

func (o *OrderRecord) GetAvgFillPrice() float64 {
	trades := o.GetTrades()
	if len(trades) == 0 {
		return 0
	}

	total := 0.0
	for _, trade := range trades {
		total += trade.Price
	}

	return total / float64(len(trades))
}

func (o *OrderRecord) FetchOrderRecordFromDB(db *gorm.DB, playgroundId uuid.UUID) (*OrderRecord, error) {
	var orderRec OrderRecord
	if result := db.First(&orderRec, "external_id = ? AND playground_id = ?", o.ID, playgroundId); result.Error != nil {
		return nil, fmt.Errorf("failed to fetch order record from db: %w", result.Error)
	}

	return &orderRec, nil
}

func (o *OrderRecord) Validate() error {
	if err := o.LiveAccountType.Validate(); err != nil {
		return fmt.Errorf("OrderRecord: invalid live account type: %w", err)
	}

	return nil
}

func CopyOrderRecord(playgroundID uuid.UUID, orderID uint, from *OrderRecord, liveAccountType LiveAccountType) *OrderRecord {
	record, err := NewOrderRecord(
		orderID,
		from.ExternalOrderID,
		from.ClientRequestID,
		playgroundID,
		from.Class,
		liveAccountType,
		from.Timestamp,
		from.Symbol,
		from.Side,
		from.AbsoluteQuantity,
		from.OrderType,
		from.Duration,
		from.RequestedPrice,
		from.Price,
		from.StopPrice,
		from.Status,
		from.Tag,
		from.CloseOrderId,
		from.IsSystemOrder,
		from.Attributes,
		from.PreviousBalance,
	)

	if err != nil {
		panic(fmt.Sprintf("CopyOrderRecord: failed to copy order record: %v", err))
	}

	return record
}

func NewOrderRecord(id uint, external_order_id *uint, client_request_id *string, playgroundId uuid.UUID, class OrderRecordClass, accountType LiveAccountType, createDate time.Time, symbol string, side TradierOrderSide, quantity float64, orderType OrderRecordType, duration OrderRecordDuration, requestedPrice float64, price, stopPrice *float64, status OrderRecordStatus, tag string, closeOrderId *uint, isSystemOrder bool, attributes map[string]string, previousBalance *float64) (*OrderRecord, error) {
	order := &OrderRecord{
		Model: gorm.Model{ID: id},
	}

	err := PopulateOrderRecord(
		order,
		external_order_id,
		client_request_id,
		playgroundId,
		symbol,
		class,
		accountType,
		createDate,
		side,
		quantity,
		orderType,
		duration,
		requestedPrice,
		price,
		stopPrice,
		status,
		tag,
		closeOrderId,
		isSystemOrder,
		attributes,
		previousBalance,
	)

	if err != nil {
		return nil, fmt.Errorf("NewOrderRecord: %w", err)
	}

	return order, nil
}

func PopulateOrderRecord(order *OrderRecord, external_order_id *uint, client_request_id *string, playgroundId uuid.UUID, symbol string, class OrderRecordClass, accountType LiveAccountType, createDate time.Time, side TradierOrderSide, quantity float64, orderType OrderRecordType, duration OrderRecordDuration, requestedPrice float64, price, stopPrice *float64, status OrderRecordStatus, tag string, closeOrderId *uint, isSystemOrder bool, attributes map[string]string, previousBalance *float64) error {
	instrument, err := eventmodels.NewInstrument(string(class), symbol)
	if err != nil {
		return fmt.Errorf("makeOrderRecord: failed to create instrument for class %s and symbol %s: %w", class, symbol, err)
	}

	order.ExternalOrderID = external_order_id
	order.ClientRequestID = client_request_id
	order.PlaygroundID = playgroundId
	order.Class = class
	order.LiveAccountType = accountType
	order.Timestamp = createDate
	order.Symbol = symbol
	order.instrument = instrument
	order.Side = side
	order.AbsoluteQuantity = quantity
	order.OrderType = orderType
	order.Duration = duration
	order.RequestedPrice = requestedPrice
	order.Price = price
	order.StopPrice = stopPrice
	order.Tag = tag
	order.Status = status
	order.Trades = []*TradeRecord{}
	order.ClosedBy = []*TradeRecord{}
	order.Closes = []*OrderRecord{}
	order.CloseOrderId = closeOrderId
	order.IsAdjustment = false
	order.IsSystemOrder = isSystemOrder
	order.Attributes = attributes
	order.PreviousBalance = previousBalance

	return nil
}
