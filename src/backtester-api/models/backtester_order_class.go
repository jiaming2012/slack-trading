package models

import "fmt"

type OrderRecordClass string

const (
	OrderRecordClassEquity  OrderRecordClass = "equity"
	OrderRecordClassOption  OrderRecordClass = "option"
	OrderRecordClassUnknown OrderRecordClass = "unknown"
)

func (c OrderRecordClass) Validate() error {
	switch c {
	case OrderRecordClassEquity:
		return nil
	case OrderRecordClassOption:
		return nil
	default:
		return fmt.Errorf("invalid order class: %s", c)
	}
}
