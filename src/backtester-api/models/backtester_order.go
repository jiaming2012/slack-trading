package models

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// type BacktesterOrder struct {
// 	PlaygroundID     uuid.UUID               `json:"-"`
// 	ID               uint                    `json:"id"`
// 	ExternalOrderID  *uint                   `json:"external_order_id,omitempty"`
// 	Class            OrderRecordClass    `json:"class"`
// 	Symbol           eventmodels.Instrument  `json:"symbol"`
// 	Side             TradierOrderSide        `json:"side"`
// 	AbsoluteQuantity float64                 `json:"quantity"`
// 	Type             OrderRecordType     `json:"type"`
// 	Duration         OrderRecordDuration `json:"duration"`
// 	Price            *float64                `json:"price,omitempty"`
// 	RequestedPrice   float64                 `json:"requested_price"`
// 	StopPrice        *float64                `json:"stop_price,omitempty"`
// 	Tag              string                  `json:"tag"`
// 	Trades           []*TradeRecord          `json:"trades"`
// 	Status           OrderRecordStatus   `json:"status"`
// 	RejectReason     *string                 `json:"reject_reason,omitempty"`
// 	CreateDate       time.Time               `json:"create_date"`
// 	IsClose          bool                    `json:"is_close"`
// 	CloseOrderId     *uint                   `json:"close_order_id,omitempty"`
// 	Reconciles       []*OrderRecord          `json:"reconciles"`
// 	ClosedBy         []*TradeRecord          `json:"closed_by"`
// 	Closes           []*OrderRecord      `json:"closes"`
// }

// func (o *OrderRecord) Cancel() {
// 	o.Status = OrderRecordStatusCancelled
// }

// func (o *OrderRecord) Reject(err error) {
// 	reason := err.Error()
// 	o.RejectReason = &reason
// 	o.Status = OrderRecordStatusRejected
// }

// func (o *OrderRecord) GetQuantity() float64 {
// 	if o.Side == TradierOrderSideSell || o.Side == TradierOrderSideSellShort {
// 		return -o.AbsoluteQuantity
// 	}

// 	return o.AbsoluteQuantity
// }

// func (o *OrderRecord) Fill(trade *TradeRecord) (bool, error) {
// 	if !o.Status.IsTradingAllowed() {
// 		return false, ErrTradingNotAllowed
// 	}

// 	if trade.Price <= 0 {
// 		return false, fmt.Errorf("trade price must be greater than 0")
// 	}

// 	filledQuantity := 0.0
// 	for _, t := range o.Trades {
// 		filledQuantity += t.Quantity
// 	}

// 	if trade.Quantity == 0 {
// 		return false, fmt.Errorf("trade quantity must be non-zero")
// 	}

// 	if math.Abs(trade.Quantity+filledQuantity) > o.AbsoluteQuantity {
// 		return false, fmt.Errorf("trade quantity exceeds order quantity")
// 	}

// 	o.Trades = append(o.Trades, trade)

// 	orderIsFilled := false
// 	if o.GetFilledVolume() == o.GetQuantity() {
// 		o.Status = OrderRecordStatusFilled
// 		orderIsFilled = true
// 	}

// 	return orderIsFilled, nil
// }

// func (o *OrderRecord) GetStatus() OrderRecordStatus {
// 	if !o.Status.IsTradingAllowed() {
// 		return o.Status
// 	}

// 	if len(o.Trades) == 0 {
// 		return OrderRecordStatusOpen
// 	}

// 	filledQuantity := o.GetFilledVolume()

// 	if filledQuantity == o.GetQuantity() {
// 		return OrderRecordStatusFilled
// 	}

// 	return OrderRecordStatusPartiallyFilled
// }

// func (o *OrderRecord) GetRemainingOpenQuantity() (float64, error) {
// 	closedQty := 0.0
// 	for _, trade := range o.ClosedBy {
// 		closedQty += trade.Quantity
// 	}

// 	if o.Side == TradierOrderSideBuy {
// 		return math.Max(0, o.GetFilledVolume()+closedQty), nil
// 	} else if o.Side == TradierOrderSideSellShort {
// 		return math.Min(0, o.GetFilledVolume()+closedQty), nil
// 	} else {
// 		return 0, fmt.Errorf("GetRemainingOpenQuantity: unsupported order side")
// 	}
// }

// func (o *OrderRecord) GetFilledVolume() float64 {
// 	filledVolume := 0.0
// 	for _, trade := range o.Trades {
// 		filledVolume += trade.Quantity
// 	}

// 	return filledVolume
// }

// func (o *OrderRecord) GetAvgFillPrice() float64 {
// 	if len(o.Trades) == 0 {
// 		return 0
// 	}

// 	total := 0.0
// 	for _, trade := range o.Trades {
// 		total += trade.Price
// 	}

// 	return total / float64(len(o.Trades))
// }

// func (o *OrderRecord) FetchOrderRecordFromDB(db *gorm.DB, playgroundId uuid.UUID) (*OrderRecord, error) {
// 	var orderRec OrderRecord
// 	if result := db.First(&orderRec, "external_id = ? AND playground_id = ?", o.ID, playgroundId); result.Error != nil {
// 		return nil, fmt.Errorf("failed to fetch order record from db: %w", result.Error)
// 	}

// 	return &orderRec, nil
// }

func fetchOrderRecordFromDB(db *gorm.DB, playgroundId uuid.UUID, orderId uint) (*OrderRecord, error) {
	var orderRec OrderRecord
	if result := db.First(&orderRec, "external_id = ? AND playground_id = ?", orderId, playgroundId); result.Error != nil {
		return nil, fmt.Errorf("failed to fetch order record from db: %w", result.Error)
	}

	return &orderRec, nil
}

// func (o *OrderRecord) ToOrderRecord(playgroundId uuid.UUID, accountType LiveAccountType) *OrderRecord {
// 	externalOrderID := uint(0)
// 	if o.ExternalOrderID != nil {
// 		externalOrderID = *o.ExternalOrderID
// 	}

// 	return &OrderRecord{
// 		PlaygroundID:     playgroundId,
// 		ExternalOrderID:  externalOrderID,
// 		Class:            string(o.Class),
// 		AccountType:      string(accountType),
// 		Symbol:           o.Symbol.GetTicker(),
// 		Side:             string(o.Side),
// 		AbsoluteQuantity: o.AbsoluteQuantity,
// 		OrderType:        string(o.Type),
// 		Duration:         string(o.Duration),
// 		Price:            o.Price,
// 		RequestedPrice:   o.RequestedPrice,
// 		RejectReason:     o.RejectReason,
// 		StopPrice:        o.StopPrice,
// 		Status:           string(o.Status),
// 		Tag:              o.Tag,
// 		Timestamp:        o.Timestamp,
// 		CloseOrderId:     o.CloseOrderId,
// 		Trades:           o.Trades,
// 	}
// }

// func NewOrderRecord(id uint, external_order_id *uint, playgroundId uuid.UUID, class OrderRecordClass, createDate time.Time, symbol eventmodels.Instrument, side TradierOrderSide, quantity float64, orderType OrderRecordType, duration OrderRecordDuration, requestedPrice float64, price, stopPrice *float64, status OrderRecordStatus, tag string, closeOrderId *uint) *OrderRecord {
// 	return &OrderRecord{
// 		ID:               id,
// 		ExternalOrderID:  external_order_id,
// 		PlaygroundID:     playgroundId,
// 		Class:            class,
// 		CreateDate:       createDate,
// 		Symbol:           symbol,
// 		Side:             side,
// 		AbsoluteQuantity: quantity,
// 		Type:             orderType,
// 		Duration:         duration,
// 		RequestedPrice:   requestedPrice,
// 		Price:            price,
// 		StopPrice:        stopPrice,
// 		Tag:              tag,
// 		Status:           status,
// 		Trades:           []*TradeRecord{},
// 		ClosedBy:         []*TradeRecord{},
// 		Closes:           []*OrderRecord{},
// 		CloseOrderId:     closeOrderId,
// 	}
// }
