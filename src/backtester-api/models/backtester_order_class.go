package models

import "fmt"

type OrderRecordClass string

const (
	OrderRecordClassEquity OrderRecordClass = "equity"
)

func (c OrderRecordClass) Validate() error {
	switch c {
	case OrderRecordClassEquity:
		return nil
	default:
		return fmt.Errorf("invalid order class: %s", c)
	}
}
