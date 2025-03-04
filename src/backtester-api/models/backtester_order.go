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
	PlaygroundID     uuid.UUID               `json:"-"`
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
	CloseOrderId     *uint                   `json:"close_order_id,omitempty"`
	Reconciles       []*OrderRecord          `json:"reconciles"`
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

func (o *BacktesterOrder) Fill(trade *TradeRecord) (bool, error) {
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

	if math.Abs(trade.Quantity+filledQuantity) > o.AbsoluteQuantity {
		return false, fmt.Errorf("trade quantity exceeds order quantity")
	}

	o.Trades = append(o.Trades, trade)

	orderIsFilled := false
	if o.GetFilledVolume() == o.GetQuantity() {
		o.Status = BacktesterOrderStatusFilled
		orderIsFilled = true
	}

	return orderIsFilled, nil
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
	Reconciles   []*OrderRecord
	PlaygroundId *uuid.UUID
	ClosedBy     []*TradeRecord
}

func (o *BacktesterOrder) ToOrderRecord(playgroundId uuid.UUID, accountType LiveAccountType) *OrderRecord {
	return &OrderRecord{
		PlaygroundID:    playgroundId,
		ExternalOrderID: o.ID,
		Class:           string(o.Class),
		AccountType:     string(accountType),
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
		CloseOrderId:    o.CloseOrderId,
		Trades:          o.Trades,
	}
}

func (o *BacktesterOrder) UpdateOrderRecord(tx *gorm.DB, playgroundId uuid.UUID, liveAccountType *LiveAccountType) (*OrderRecord, []*UpdateOrderRecordRequest, error) {
	if liveAccountType == nil {
		typ := LiveAccountTypeSimulator
		liveAccountType = &typ
	}

	orderRec := o.ToOrderRecord(playgroundId, *liveAccountType)

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
			Field:        "closed_by",
			OrderRecord:  orderRec,
			ClosedBy:     o.ClosedBy,
			PlaygroundId: &playgroundId,
		})
	}

	// create update order request for update reconciled by
	if len(o.Reconciles) > 0 {
		updateOrderRequests = append(updateOrderRequests, &UpdateOrderRecordRequest{
			Field:        "reconciles",
			OrderRecord:  orderRec,
			Reconciles:   o.Reconciles,
			PlaygroundId: &playgroundId,
		})
	}

	return orderRec, updateOrderRequests, nil
}

func NewBacktesterOrder(id uint, playgroundId uuid.UUID, class BacktesterOrderClass, createDate time.Time, symbol eventmodels.Instrument, side TradierOrderSide, quantity float64, orderType BacktesterOrderType, duration BacktesterOrderDuration, requestedPrice float64, price, stopPrice *float64, status BacktesterOrderStatus, tag string, closeOrderId *uint) *BacktesterOrder {
	return &BacktesterOrder{
		ID:               id,
		PlaygroundID:     playgroundId,
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
		CloseOrderId:     closeOrderId,
	}
}

func CopyBacktesterOrder(from *BacktesterOrder) *BacktesterOrder {
	return NewBacktesterOrder(
		from.ID,
		from.PlaygroundID,
		from.Class,
		from.CreateDate,
		from.Symbol,
		from.Side,
		from.AbsoluteQuantity,
		from.Type,
		from.Duration,
		from.RequestedPrice,
		from.Price,
		from.StopPrice,
		from.Status,
		from.Tag,
		from.CloseOrderId,
	)
}
