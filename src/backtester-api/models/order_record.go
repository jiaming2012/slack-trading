package models

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type OrderRecord struct {
	gorm.Model
	PlaygroundID     uuid.UUID              `gorm:"column:playground_id;type:uuid;not null;index:idx_playground_order" copier:"must,nopanic"`
	ClientRequestID  *string                `gorm:"column:client_request_id;type:text" copier:"must,nopanic"`
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
	Closes           []*OrderRecord         `gorm:"many2many:order_closes" copier:"must,nopanic"`
	ClosedBy         []*TradeRecord         `gorm:"many2many:trade_closed_by" copier:"must,nopanic"`
	Reconciles       []*OrderRecord         `gorm:"many2many:order_reconciles" copier:"must,nopanic"`
	Trades           []*TradeRecord         `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" copier:"must,nopanic"`
	ReconcileTrades  []*TradeRecord         `gorm:"foreignKey:ReconcileOrderID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" copier:"must,nopanic"`
	instrument       eventmodels.Instrument `gorm:"-" copier:"must,nopanic"`
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
	if o.Side == TradierOrderSideSell || o.Side == TradierOrderSideSellShort {
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

	if trade.Price <= 0 {
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

	if math.Abs(trade.Quantity+filledQuantity) > o.AbsoluteQuantity {
		return false, fmt.Errorf("trade quantity exceeds order quantity")
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
		switch o.Class {
		case OrderRecordClassEquity:
			o.instrument = eventmodels.NewStockSymbol(o.Symbol)
		default:
			return fmt.Errorf("unsupported order record class: %s", o.Class)
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
		return OrderRecordStatusOpen
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

	if o.Side == TradierOrderSideBuy {
		return math.Max(0, o.GetFilledVolume()+closedQty), nil
	} else if o.Side == TradierOrderSideSellShort {
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
	return NewOrderRecord(
		orderID,
		from.ExternalOrderID,
		from.ClientRequestID,
		playgroundID,
		from.Class,
		liveAccountType,
		from.Timestamp,
		from.instrument,
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
	)
}

func NewOrderRecord(id uint, external_order_id *uint, client_request_id *string, playgroundId uuid.UUID, class OrderRecordClass, accountType LiveAccountType, createDate time.Time, symbol eventmodels.Instrument, side TradierOrderSide, quantity float64, orderType OrderRecordType, duration OrderRecordDuration, requestedPrice float64, price, stopPrice *float64, status OrderRecordStatus, tag string, closeOrderId *uint) *OrderRecord {
	o := &OrderRecord{
		ExternalOrderID:  external_order_id,
		ClientRequestID:  client_request_id,
		PlaygroundID:     playgroundId,
		Class:            class,
		LiveAccountType:  accountType,
		Timestamp:        createDate,
		Symbol:           symbol.GetTicker(),
		instrument:       symbol,
		Side:             side,
		AbsoluteQuantity: quantity,
		OrderType:        orderType,
		Duration:         duration,
		RequestedPrice:   requestedPrice,
		Price:            price,
		StopPrice:        stopPrice,
		Tag:              tag,
		Status:           status,
		Trades:           []*TradeRecord{},
		ClosedBy:         []*TradeRecord{},
		Closes:           []*OrderRecord{},
		CloseOrderId:     closeOrderId,
		IsAdjustment:     false,
	}

	if id != 0 {
		o.ID = id
	}

	return o
}
