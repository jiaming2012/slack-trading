package eventmodels

import (
	"fmt"
	"log"
	"math"
	"time"
)

type TradierOrder struct {
	ID                        uint                 `json:"id"`
	Type                      string               `json:"type"`
	Symbol                    string               `json:"symbol"`
	Side                      string               `json:"side"`
	AbsoluteQuantity          float64              `json:"quantity"`
	Status                    string               `json:"status"`
	Duration                  string               `json:"duration"`
	Price                     float64              `json:"price"`
	AvgFillPrice              float64              `json:"avg_fill_price"`
	AbsoluteExecQuantity      float64              `json:"exec_quantity"`
	LastFillPrice             float64              `json:"last_fill_price"`
	AbsoluteLastFillQuantity  float64              `json:"last_fill_quantity"`
	AbsoluteRemainingQuantity float64              `json:"remaining_quantity"`
	CreateDate                time.Time            `json:"create_date"`
	TransactionDate           time.Time            `json:"transaction_date"`
	Class                     string               `json:"class"`
	OptionSymbol              *string              `json:"option_symbol"`
	Leg                       []TradierOrderLegDTO `json:"leg"`
	Strategy                  string               `json:"strategy"`
	ReasonDescription         *string              `json:"reason_description"`
	Tag                       string               `json:"tag"`
}

func (o TradierOrder) GetExecFillQuantity() float64 {
	if o.Side == "buy" || o.Side == "buy_to_cover" {
		return math.Abs(o.AbsoluteExecQuantity)
	} else if o.Side == "sell" || o.Side == "sell_short" {
		return -math.Abs(o.AbsoluteExecQuantity)
	} else {
		log.Fatalf("TradierOrder.GetExecFillQuantity: invalid side: %s", o.Side)
	}

	return 0
}

func (o TradierOrder) GetLastFillQuantity() float64 {
	if o.Side == "buy" || o.Side == "buy_to_cover" {
		return o.AbsoluteLastFillQuantity
	} else if o.Side == "sell" || o.Side == "sell_short" {
		return -o.AbsoluteLastFillQuantity
	} else {
		log.Fatalf("TradierOrder.GetLastFillQuantity: invalid side: %s", o.Side)
	}

	return 0
}

func (o TradierOrder) GetLegs(option1 *OptionSymbolComponents, option2 *OptionSymbolComponents) (*TradierOrderLegDTO, *TradierOrderLegDTO, error) {
	if len(o.Leg) != 2 {
		return nil, nil, fmt.Errorf("TradierOrder.GetLeg: invalid leg count: %d", len(o.Leg))
	}

	if option1.OptionType != option2.OptionType {
		return nil, nil, fmt.Errorf("TradierOrder.GetLeg: option types do not match: %s, %s", option1.OptionType, option2.OptionType)
	}

	if o.Leg[0].Side == "sell_to_open" && o.Leg[1].Side == "buy_to_open" {
		return &o.Leg[0], &o.Leg[1], nil
	}

	if o.Leg[0].Side == "buy_to_open" && o.Leg[1].Side == "sell_to_open" {
		return &o.Leg[1], &o.Leg[0], nil
	}

	return nil, nil, fmt.Errorf("TradierOrder.GetLeg: invalid sides: %s, %s", o.Leg[0].Side, o.Leg[1].Side)
}

func (o TradierOrder) String() string {
	var symbol string
	if o.OptionSymbol != nil {
		symbol = *o.OptionSymbol
	} else {
		symbol = o.Symbol
	}

	timestamp := o.CreateDate.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("ID (%d), Type: %s, Symbol: %s, Side: %s, Status: %s, AvgFillPrice: %.2f, ExecQuantity: %.0f, Class: %s, CreatedAt: %v", o.ID, o.Type, symbol, o.Side, o.Status, o.AvgFillPrice, o.AbsoluteExecQuantity, o.Class, timestamp)
}
