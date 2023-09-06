package eventdto

import (
	"encoding/json"
	"fmt"
	"time"
)

type Timeframe string

const (
	M1  Timeframe = "m1"
	M5  Timeframe = "m5"
	M15 Timeframe = "m15"
	H1  Timeframe = "h1"
	H4  Timeframe = "h4"
	D1  Timeframe = "d1"
	W1  Timeframe = "w1"
)

func (t Timeframe) Validate() (time.Duration, error) {
	switch t {
	case M1:
		return time.Minute, nil
	case M5:
		return 5 * time.Minute, nil
	case M15:
		return 15 * time.Minute, nil
	case H1:
		return 1 * time.Hour, nil
	case H4:
		return 4 * time.Hour, nil
	case D1:
		return 24 * time.Hour, nil
	case W1:
		return 168 * time.Hour, nil
	default:
		return time.Duration(0), fmt.Errorf("invalid timeframe: %v", t)
	}
}

type Header struct {
	Symbol           string    `json:"symbol"`
	Timeframe        Timeframe `json:"timeframe"`
	Signal           string    `json:"signal"`
	PriceActionEvent string    `json:"price_action_event"`
}

type TrendspiderWebhook struct {
	Header Header          `json:"header"`
	Data   json.RawMessage `json:"data"`
}

// SupportBreakSignal represents webhook signal support_break
type SupportBreakSignal struct {
	Price string `json:"price"`
}

// ResistanceBreakSignal represents webhook signal resistance_break
type ResistanceBreakSignal struct {
	Price string `json:"price"`
}

// TrendlineBreakSignal represents webhook signal trendline_break
type TrendlineBreakSignal struct {
	Price string `json:"price"`
}
