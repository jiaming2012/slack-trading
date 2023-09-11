package models

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"math"
	"strings"
	"sync"
)

type PriceLevels struct {
	Values []*PriceLevel
}

func (levels PriceLevels) GetByIndex(index int) (*PriceLevel, error) {
	if index < 0 {
		return nil, fmt.Errorf("Strategy.GetPriceLevelByIndex: index must be greater than or equal to zero. Found %v", index)
	}

	if index >= len(levels.Values) {
		return nil, fmt.Errorf("Strategy.GetPriceLevelByIndex: index must be less than total number of price levels of %v", len(levels.Values))
	}

	return levels.Values[index], nil
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

type PriceLevel struct {
	Price                float64
	MinimumTradeDistance float64 // the minimum distance of the requested price of two trades in the same price band
	MaxNoOfTrades        int
	AllocationPercent    float64 // the amount of Account.Balance allocated to this price level
	Trades               *Trades
	StopLoss             float64
	mutex                sync.Mutex
}

func (p *PriceLevel) canAddTrade(trade *Trade) error {
	if trade.Type == TradeTypeBuy || trade.Type == TradeTypeSell {
		openTrades := p.Trades.OpenTrades()
		for _, open := range *openTrades {
			if math.Abs(trade.RequestedPrice-open.RequestedPrice) < p.MinimumTradeDistance {
				return fmt.Errorf("PriceLevel.canAddTrade: request price of %v is too close to request price of previously open trade %v: %w", trade.RequestedPrice, open.RequestedPrice, PriceLevelMinimumDistanceNotSatisfiedError)
			}
		}
	}

	return nil
}

func (p *PriceLevel) Add(trade *Trade) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.canAddTrade(trade); err != nil {
		return err
	}

	p.Trades.Add(trade)

	return nil
}

func (p *PriceLevel) Validate() error {
	if p.MinimumTradeDistance < 0 {
		return PriceLevelMinimumDistanceNotSatisfiedError
	}

	if p.AllocationPercent < 0 || p.AllocationPercent > 1 {
		return InvalidAllocationPercentErr
	}

	if p.AllocationPercent > 0 && p.StopLoss <= 0 {
		return NonPositiveStopLoss
	}

	if p.MaxNoOfTrades < 0 {
		return InvalidMaxTradesErr
	}

	if p.Price < 0 {
		return NegativePriceErr
	}

	return nil
}

func (p *PriceLevel) NewTradesRemaining() (int, TradeType) {
	buysCount := 0
	buyVolume := 0.0
	sellVolume := 0.0
	sellsCount := 0
	closedBuyVolume := 0.0
	closedSellVolume := 0.0
	diff := 0

	for _, t := range *p.Trades {
		if t.Type == TradeTypeClose {
			if t.ExecutedVolume < 0 {
				closedBuyVolume += math.Abs(t.ExecutedVolume)
			} else if t.ExecutedVolume > 0 {
				closedSellVolume += t.ExecutedVolume
			}
		}
	}

	// todo: null pointer check
	for _, t := range *p.Trades {
		if t.Type == TradeTypeBuy {
			executedVolume := t.ExecutedVolume
			if closedBuyVolume > 0 {
				executedVolume -= math.Min(t.ExecutedVolume, closedBuyVolume)
				closedBuyVolume = math.Max(closedBuyVolume-t.ExecutedVolume, 0)
			}

			if executedVolume > 0 {
				buysCount += 1
				buyVolume += executedVolume
			}
		} else if t.Type == TradeTypeSell {
			executedVolume := math.Abs(t.ExecutedVolume)
			if closedSellVolume > 0 {
				executedVolume -= math.Min(t.ExecutedVolume, closedBuyVolume)
				closedSellVolume = math.Max(closedBuyVolume-t.ExecutedVolume, 0)
			}

			if executedVolume > 0 {
				sellsCount += 1
				sellVolume += executedVolume
			}
		}
	}

	var side TradeType
	if buysCount > 0 {
		side = TradeTypeBuy
		diff = buysCount
	} else if sellsCount > 0 {
		side = TradeTypeSell
		diff = sellsCount
	} else {
		side = TradeTypeNone
		diff = 0
	}

	return p.MaxNoOfTrades - diff, side
}

// todo: reeval if we need this
//type BalanceLevelStats struct {
//	TotalBalance float64
//	UsedBalance  float64
//}
