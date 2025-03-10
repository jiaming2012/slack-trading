package models

import "fmt"

type OrderRecordType string

const (
	Market    OrderRecordType = "market"
	Limit     OrderRecordType = "limit"
	Stop      OrderRecordType = "stop"
	StopLimit OrderRecordType = "stop_limit"
)

func (t OrderRecordType) Validate() error {
	switch t {
	case Market, Limit, Stop, StopLimit:
		return nil
	default:
		return fmt.Errorf("invalid order type: %s", t)
	}
}
