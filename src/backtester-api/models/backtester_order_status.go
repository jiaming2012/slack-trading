package models

type OrderRecordStatus string

const (
	OrderRecordStatusNew             OrderRecordStatus = "new"
	OrderRecordStatusPending         OrderRecordStatus = "pending"
	OrderRecordStatusPartiallyFilled OrderRecordStatus = "partially_filled"
	OrderRecordStatusFilled          OrderRecordStatus = "filled"
	OrderRecordStatusExpired         OrderRecordStatus = "expired"
	OrderRecordStatusCanceled        OrderRecordStatus = "canceled"
	OrderRecordStatusRejected        OrderRecordStatus = "rejected"
)

func (status OrderRecordStatus) IsTradingAllowed() bool {
	return status == OrderRecordStatusPending || status == OrderRecordStatusNew || status == OrderRecordStatusPartiallyFilled
}

func (status OrderRecordStatus) IsFilled() bool {
	return status == OrderRecordStatusFilled || status == OrderRecordStatusPartiallyFilled
}
