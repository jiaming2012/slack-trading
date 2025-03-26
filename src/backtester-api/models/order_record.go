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
	PlaygroundID     uuid.UUID              `gorm:"column:playground_id;type:uuid;not null;index:idx_playground_order"`
	LiveAccountType  LiveAccountType        `gorm:"column:account_type;type:text;not null"`
	ExternalOrderID  *uint                  `gorm:"column:external_id;index:idx_external_order_id"`
	Class            OrderRecordClass       `gorm:"column:class;type:text;not null"`
	Symbol           string                 `gorm:"column:symbol;type:text;not null"`
	Side             TradierOrderSide       `gorm:"column:side;type:text;not null"`
	AbsoluteQuantity float64                `gorm:"column:quantity;type:numeric;not null"`
	OrderType        OrderRecordType        `gorm:"column:order_type;type:text;not null"`
	Duration         OrderRecordDuration    `gorm:"column:duration;type:text;not null"`
	Price            *float64               `gorm:"column:price;type:numeric"`
	RequestedPrice   float64                `gorm:"column:requested_price;type:numeric"`
	StopPrice        *float64               `gorm:"column:stop_price;type:numeric"`
	Status           OrderRecordStatus      `gorm:"column:status;type:text;not null"`
	RejectReason     *string                `gorm:"column:reject_reason;type:text"`
	Tag              string                 `gorm:"column:tag;type:text"`
	Timestamp        time.Time              `gorm:"column:timestamp;type:timestamptz;not null"`
	IsAdjustment     bool                   `gorm:"column:is_adjustment"`
	IsClose          bool                   `gorm:"-"`
	CloseOrderId     *uint                  `gorm:"column:close_order_id"`
	Closes           []*OrderRecord         `gorm:"many2many:order_closes"`
	ClosedBy         []*TradeRecord         `gorm:"many2many:trade_closed_by"`
	Reconciles       []*OrderRecord         `gorm:"many2many:order_reconciles"`
	Trades           []*TradeRecord         `gorm:"foreignKey:OrderID"`
	instrument       eventmodels.Instrument `gorm:"-"`
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
	o.Status = OrderRecordStatusCancelled
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

func (o *OrderRecord) Fill(trade *TradeRecord) (bool, error) {
	if !o.Status.IsTradingAllowed() {
		return false, ErrTradingNotAllowed
	}

	if trade.Price <= 0 {
		return false, fmt.Errorf("trade price must be greater than 0")
	}

	filledQuantity := 0.0
	for _, t := range o.Trades {
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

	o.Trades = append(o.Trades, trade)

	orderIsFilled := false
	if o.IsFilled() {
		o.Status = OrderRecordStatusFilled
		orderIsFilled = true
	}

	return orderIsFilled, nil
}

func (o *OrderRecord) SetStatus(status OrderRecordStatus) {
	o.Status = status
}

func (o *OrderRecord) GetStatus() OrderRecordStatus {
	if !o.Status.IsTradingAllowed() {
		return o.Status
	}

	if len(o.Trades) == 0 {
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
	for _, trade := range o.Trades {
		filledVolume += trade.Quantity
	}

	return filledVolume
}

func (o *OrderRecord) GetAvgFillPrice() float64 {
	if len(o.Trades) == 0 {
		return 0
	}

	total := 0.0
	for _, trade := range o.Trades {
		total += trade.Price
	}

	return total / float64(len(o.Trades))
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

func NewOrderRecord(id uint, external_order_id *uint, playgroundId uuid.UUID, class OrderRecordClass, accountType LiveAccountType, createDate time.Time, symbol eventmodels.Instrument, side TradierOrderSide, quantity float64, orderType OrderRecordType, duration OrderRecordDuration, requestedPrice float64, price, stopPrice *float64, status OrderRecordStatus, tag string, closeOrderId *uint) *OrderRecord {
	o := &OrderRecord{
		ExternalOrderID:  external_order_id,
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
