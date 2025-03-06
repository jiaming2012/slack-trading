package models

import "fmt"

type OrderRecordDuration string

const (
	Day        OrderRecordDuration = "day"
	GTC        OrderRecordDuration = "gtc"
	PreMarket  OrderRecordDuration = "pre"
	PostMarket OrderRecordDuration = "post"
)

func (d OrderRecordDuration) Validate() error {
	switch d {
	case Day, GTC, PreMarket, PostMarket:
		return nil
	default:
		return fmt.Errorf("invalid order duration: %s", d)
	}
}
