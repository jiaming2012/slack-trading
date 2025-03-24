package models

type OrderRecordStatus string

const (
	OrderRecordStatusOpen            OrderRecordStatus = "open"
	OrderRecordStatusPending         OrderRecordStatus = "pending"
	OrderRecordStatusPartiallyFilled OrderRecordStatus = "partially_filled"
	OrderRecordStatusFilled          OrderRecordStatus = "filled"
	OrderRecordStatusExpired         OrderRecordStatus = "expired"
	OrderRecordStatusCancelled       OrderRecordStatus = "canceled"
	OrderRecordStatusRejected        OrderRecordStatus = "rejected"
)

func (status OrderRecordStatus) IsTradingAllowed() bool {
	return status == OrderRecordStatusPending || status == OrderRecordStatusOpen || status == OrderRecordStatusPartiallyFilled
}

func (status OrderRecordStatus) IsFilled() bool {
	return status == OrderRecordStatusFilled || status == OrderRecordStatusPartiallyFilled
}
