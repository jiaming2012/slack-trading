package models

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterOrder struct {
	ID               uint                    `json:"id"`
	Class            BacktesterOrderClass    `json:"class"`
	Symbol           eventmodels.Instrument  `json:"symbol"`
	Side             TradierOrderSide        `json:"side"`
	AbsoluteQuantity float64                 `json:"quantity"`
	Type             BacktesterOrderType     `json:"type"`
	Duration         BacktesterOrderDuration `json:"duration"`
	Price            *float64                `json:"price,omitempty"`
	RequestedPrice   float64                 `json:"requested_price"`
	StopPrice        *float64                `json:"stop_price,omitempty"`
	Tag              string                  `json:"tag"`
	Trades           []*TradeRecord          `json:"trades"`
	Status           BacktesterOrderStatus   `json:"status"`
	RejectReason     *string                 `json:"reject_reason,omitempty"`
	CreateDate       time.Time               `json:"create_date"`
	IsClose          bool                    `json:"is_close"`
	ClosedBy         []*TradeRecord          `json:"closed_by"`
	Closes           []*BacktesterOrder      `json:"closes"`
}

func (o *BacktesterOrder) Cancel() {
	o.Status = BacktesterOrderStatusCancelled
}

func (o *BacktesterOrder) Reject(err error) {
	reason := err.Error()
	o.RejectReason = &reason
	o.Status = BacktesterOrderStatusRejected
}

func (o *BacktesterOrder) GetQuantity() float64 {
	if o.Side == TradierOrderSideSell || o.Side == TradierOrderSideSellShort {
		return -o.AbsoluteQuantity
	}

	return o.AbsoluteQuantity
}

func (o *BacktesterOrder) Fill(trade *TradeRecord) error {
	if !o.Status.IsTradingAllowed() {
		return ErrTradingNotAllowed
	}

	if trade.Price <= 0 {
		return fmt.Errorf("trade price must be greater than 0")
	}

	filledQuantity := 0.0
	for _, t := range o.Trades {
		filledQuantity += t.Quantity
	}

	if trade.Quantity == 0 {
		return fmt.Errorf("trade quantity must be non-zero")
	}

	if math.Abs(trade.Quantity+filledQuantity) > o.AbsoluteQuantity {
		return fmt.Errorf("trade quantity exceeds order quantity")
	}

	o.Trades = append(o.Trades, trade)

	if o.GetFilledVolume() == o.GetQuantity() {
		o.Status = BacktesterOrderStatusFilled
	}

	return nil
}

func (o *BacktesterOrder) GetStatus() BacktesterOrderStatus {
	if !o.Status.IsTradingAllowed() {
		return o.Status
	}

	if len(o.Trades) == 0 {
		return BacktesterOrderStatusOpen
	}

	filledQuantity := o.GetFilledVolume()

	if filledQuantity == o.GetQuantity() {
		return BacktesterOrderStatusFilled
	}

	return BacktesterOrderStatusPartiallyFilled
}

func (o *BacktesterOrder) GetRemainingOpenQuantity() (float64, error) {
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

func (o *BacktesterOrder) GetFilledVolume() float64 {
	filledVolume := 0.0
	for _, trade := range o.Trades {
		filledVolume += trade.Quantity
	}

	return filledVolume
}

func (o *BacktesterOrder) GetAvgFillPrice() float64 {
	if len(o.Trades) == 0 {
		return 0
	}

	total := 0.0
	for _, trade := range o.Trades {
		total += trade.Price
	}

	return total / float64(len(o.Trades))
}

func (o *BacktesterOrder) FetchOrderRecordFromDB(db *gorm.DB, playgroundId uuid.UUID) (*OrderRecord, error) {
	var orderRec OrderRecord
	if result := db.First(&orderRec, "external_id = ? AND playground_id = ?", o.ID, playgroundId); result.Error != nil {
		return nil, fmt.Errorf("failed to fetch order record from db: %w", result.Error)
	}

	return &orderRec, nil
}

func fetchOrderRecordFromDB(db *gorm.DB, playgroundId uuid.UUID, orderId uint) (*OrderRecord, error) {
	var orderRec OrderRecord
	if result := db.First(&orderRec, "external_id = ? AND playground_id = ?", orderId, playgroundId); result.Error != nil {
		return nil, fmt.Errorf("failed to fetch order record from db: %w", result.Error)
	}

	return &orderRec, nil
}

type UpdateOrderRecordRequest struct {
	Field        string
	OrderRecord  *OrderRecord
	Closes       []*BacktesterOrder
	PlaygroundId *uuid.UUID
	ClosedBy     []*TradeRecord
}

func (o *BacktesterOrder) ToOrderRecord(tx *gorm.DB, playgroundId uuid.UUID, liveAccountType *LiveAccountType) (*OrderRecord, []*UpdateOrderRecordRequest, error) {
	account_type := "simulation"
	if liveAccountType != nil {
		account_type = string(*liveAccountType)
	}

	orderRec := &OrderRecord{
		PlaygroundID:    playgroundId,
		ExternalOrderID: o.ID,
		Class:           string(o.Class),
		AccountType:     account_type,
		Symbol:          o.Symbol.GetTicker(),
		Side:            string(o.Side),
		Quantity:        o.AbsoluteQuantity,
		OrderType:       string(o.Type),
		Duration:        string(o.Duration),
		Price:           o.Price,
		RequestedPrice:  o.RequestedPrice,
		RejectReason:    o.RejectReason,
		StopPrice:       o.StopPrice,
		Status:          string(o.Status),
		Tag:             o.Tag,
		Timestamp:       o.CreateDate,
		Trades:          o.Trades,
	}

	// create update order request for update closes
	var updateOrderRequests []*UpdateOrderRecordRequest
	if len(o.Closes) > 0 {
		updateOrderRequests = append(updateOrderRequests, &UpdateOrderRecordRequest{
			Field:        "closes",
			OrderRecord:  orderRec,
			Closes:       o.Closes,
			PlaygroundId: &playgroundId,
		})
	}

	// create update order request for update closed by
	if len(o.ClosedBy) > 0 {
		updateOrderRequests = append(updateOrderRequests, &UpdateOrderRecordRequest{
			Field:       "closed_by",
			OrderRecord: orderRec,
			ClosedBy:    o.ClosedBy,
		})
	}

	return orderRec, updateOrderRequests, nil
}

func NewBacktesterOrder(id uint, class BacktesterOrderClass, createDate time.Time, symbol eventmodels.Instrument, side TradierOrderSide, quantity float64, orderType BacktesterOrderType, duration BacktesterOrderDuration, requestedPrice float64, price, stopPrice *float64, status BacktesterOrderStatus, tag string) *BacktesterOrder {
	return &BacktesterOrder{
		ID:               id,
		Class:            class,
		CreateDate:       createDate,
		Symbol:           symbol,
		Side:             side,
		AbsoluteQuantity: quantity,
		Type:             orderType,
		Duration:         duration,
		RequestedPrice:   requestedPrice,
		Price:            price,
		StopPrice:        stopPrice,
		Tag:              tag,
		Status:           status,
		Trades:           []*TradeRecord{},
		ClosedBy:         []*TradeRecord{},
		Closes:           []*BacktesterOrder{},
	}
}
