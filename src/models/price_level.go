package models

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"math"
	"strings"
)

type PriceLevels struct {
	Values []*PriceLevel
}

type PriceLevel struct {
	Price             float64
	MaxNoOfTrades     int
	AllocationPercent float64 // the amount of Account.Balance allocated to this price level
	Trades            *Trades
	StopLoss          float64
}

func (p *PriceLevel) Validate() error {
	if p.StopLoss <= 0 {
		return NonPositiveStopLoss
	}

	return nil
}

func (p *PriceLevel) NewTradesRemaining() (int, TradeType) {
	buysCount := 0
	buyVolume := 0.0
	sellVolume := 0.0
	sellsCount := 0

	// todo: null pointer check
	for _, t := range *p.Trades {
		if t.Side() == TradeTypeBuy {
			buysCount += 1
			buyVolume += t.RequestedVolume
		} else if t.Side() == TradeTypeSell {
			sellsCount += 1
			sellVolume += math.Abs(t.RequestedVolume)
		}
	}

	var diff = int(math.Abs(float64(buysCount) - float64(sellsCount)))
	if buyVolume > sellVolume {
		for _, t := range *p.Trades {
			if t.Side() == TradeTypeBuy {
				tradeVolume := t.RequestedVolume
				if sellVolume > 0 {
					delta := math.Min(sellVolume, tradeVolume)
					remainingVolume := tradeVolume - delta
					if remainingVolume > 0 {
						buysCount += 1
					}
					sellVolume -= delta
				}
				buysCount += 1
			}
		}
	} else if buyVolume < sellVolume {

	} else {

	}

	var side TradeType
	if sellVolume > buyVolume {
		side = TradeTypeSell
	} else {
		side = TradeTypeBuy
	}

	return p.MaxNoOfTrades - diff, side
}

func (levels PriceLevels) String() string {
	display := &strings.Builder{}
	p := message.NewPrinter(language.English)

	table := tablewriter.NewWriter(display)

	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetColumnSeparator("")
	display.WriteString("Price Levels:\n")

	for _, lvl := range levels.Values {
		price := fmt.Sprintf("$%s", p.Sprintf("%.2f", lvl.Price))
		noOfTrades := fmt.Sprintf("%d trades", lvl.MaxNoOfTrades)
		allocPercentage := fmt.Sprintf("%.0f%%", lvl.AllocationPercent*100)

		table.Append([]string{price, noOfTrades, allocPercentage})
	}

	table.Render()
	return display.String()
}

func (levels *PriceLevels) Validate() error {
	total := 0.0
	for _, lvl := range levels.Values {
		total += lvl.AllocationPercent
	}

	if math.Abs(1-total) > 0.001 {
		return fmt.Errorf("%w: allocation total is %v", PriceLevelsAllocationErr, total)
	}

	return nil
}

func NewPriceLevels(levels []*PriceLevel) (*PriceLevels, error) {
	for _, l := range levels {
		if err := l.Validate(); err != nil {
			return nil, err
		}
	}

	if len(levels) < 2 {
		return nil, LevelsNotSetErr
	}

	if levels[len(levels)-1].AllocationPercent != 0 {
		return nil, PriceLevelsLastAllocationErr
	}

	prevPrice := 0.0
	for _, level := range levels {
		if level.Price <= 0 {
			return nil, fmt.Errorf("NewPriceLevels: invalid price level, %v", level.Price)
		}

		if level.Price < prevPrice {
			return nil, PriceLevelsNotSortedErr
		}

		if level.AllocationPercent > 0 && level.MaxNoOfTrades <= 0 {
			return nil, NoOfTradeMustBeNonzeroErr
		}

		if level.AllocationPercent == 0 && level.MaxNoOfTrades > 0 {
			return nil, NoOfTradesMustBeZeroErr
		}

		level.Trades = &Trades{}

		prevPrice = level.Price
	}

	return &PriceLevels{
		Values: levels,
	}, nil
}

// todo: reeval if we need this
//type BalanceLevelStats struct {
//	TotalBalance float64
//	UsedBalance  float64
//}
