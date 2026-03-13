package eventmodels

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Account struct {
	Name       string      `json:"name"`
	Balance    float64     `json:"balance"`
	Strategies []*Strategy `json:"strategies"`
	Datafeed   *Datafeed   `json:"datafeed"`
	mutex      sync.Mutex  `json:"-"`
}

/* todo: change account model:
Account -> []*Strategy
Strategy -> []*PriceLevel
Trades is its own entity
  -> each trade has *Account
  -> each trade has *PriceLevel
*/

func (a *Account) String() string {
	var strategies string
	if len(a.Strategies) == 0 {
		strategies = "no strategies set"
	} else {
		var out strings.Builder

		for i, s := range a.Strategies {
			out.WriteString(fmt.Sprintf("        -> %d: %v\n", i+1, s.String()))

			out.WriteString("          Entry EntryConditions:\n")
			for i, cond := range s.EntryConditions {
				out.WriteString(fmt.Sprintf("               -> %d: %v\n", i, cond.String()))
			}
		}

		strategies = out.String()
	}

	return fmt.Sprintf("Name: %v\n     Starting Balance: $%.2f, \n     Strategies:\n%s", a.Name, a.Balance, strategies)
}

func (a *Account) GetPriceLevelTrades(openTradesOnly bool) []*TradeLevels {
	var priceLevelTrades []*TradeLevels

	for _, strategy := range a.Strategies {
		priceLevelTrades = append(priceLevelTrades, strategy.GetTradesByPriceLevel(openTradesOnly)...)
	}

	return priceLevelTrades
}

func (a *Account) GetTrades() *Trades {
	trades := Trades{}

	for _, strategy := range a.Strategies {
		trades.BulkAdd(strategy.GetTrades())
	}

	return &trades
}

func (a *Account) GetOpenTrades() *Trades {
	trades := Trades{}

	for _, strategy := range a.Strategies {
		trades.BulkAdd(strategy.GetOpenTrade())
	}

	return &trades
}

// todo: remove duplicate: in favor of CheckStopLoss()
func (a *Account) checkSL(tick Tick) []*CloseTradesRequest {
	requests := make([]*CloseTradesRequest, 0)

	for _, strategy := range a.Strategies {
		for levelIndex, level := range strategy.PriceLevels.Bands {
			if strategy.Direction == Up {
				if tick.Price <= level.StopLoss {
					if level.Trades.OpenTrades().Count() > 0 {
						requests = append(requests, &CloseTradesRequest{
							Strategy:        strategy,
							Timeframe:       nil,
							PriceLevelIndex: levelIndex,
							Reason:          "sl",
							Percent:         1.0,
						})
					}
				}
			} else if strategy.Direction == Down {
				if tick.Price >= level.StopLoss {
					if level.Trades.OpenTrades().Count() > 0 {
						requests = append(requests, &CloseTradesRequest{
							Strategy:        strategy,
							Timeframe:       nil,
							PriceLevelIndex: levelIndex,
							Reason:          "sl",
							Percent:         1.0,
						})
					}
				}
			}
		}
	}

	if len(requests) > 0 {
		return requests
	}

	return nil
}

func (a *Account) CheckStopLoss(tick Tick) ([]*CloseTradeRequestV2, error) {
	trades := a.GetOpenTrades()

	var closeTradeRequests []*CloseTradeRequestV2
	for _, t := range *trades {
		closeTradeReq, err := t.IsStopLossTriggered(tick)
		if err != nil {
			return nil, fmt.Errorf("Account.CheckStopLoss: is stop loss triggered failed: %w", err)
		}

		if closeTradeReq != nil {
			closeTradeRequests = append(closeTradeRequests, closeTradeReq)
		}
	}

	return closeTradeRequests, nil
}

func (a *Account) CheckStopOut(tick Tick) ([]*CloseTradesRequest, error) {
	for _, s := range a.Strategies {
		// todo: analyze if calling PL() so many times on each tick causes a bottleneck
		vwap, vol, realizedPL := s.GetTrades().GetTradeStatsItems()
		unrealizedPL := CalculateUnrealizedPL(vwap, vol, tick)
		pl := unrealizedPL + float64(realizedPL)

		var closeTradeRequests []*CloseTradesRequest
		if pl <= -s.Balance {
			for priceLevelIndex, level := range s.PriceLevels.Bands {
				if level.Trades.Count() > 0 {
					req, err := NewCloseTradesRequest(s, nil, priceLevelIndex, 1.0, "stop out")
					if err != nil {
						return nil, fmt.Errorf("checkStopOut: new close trades request failed: %w", err)
					}

					closeTradeRequests = append(closeTradeRequests, req)
				}
			}

			return closeTradeRequests, nil
		}
	}

	return nil, nil
}

func (a *Account) NewUUID() uuid.UUID {
	return uuid.New()
}

func (a *Account) GetCurrentTime() time.Time {
	return time.Now().UTC()
}

func (a *Account) getStrategiesBalance() float64 {
	balance := 0.0

	for _, s := range a.Strategies {
		balance += s.Balance
	}

	return balance
}

func (a *Account) AddStrategy(strategy *Strategy) error {
	for _, s := range a.Strategies {
		if strategy.Name == s.Name {
			return fmt.Errorf("Account.AddStrategy: strategy name %v already exists", strategy.Name)
		}
	}

	currentBalance := a.getStrategiesBalance()
	if strategy.Balance+currentBalance > a.Balance {
		return fmt.Errorf("Account.AddStrategy: new strategy balance of $%.2f would put account over limit of %.2f by $%.2f", strategy.Balance, a.Balance, currentBalance+strategy.Balance-a.Balance)
	}

	a.Strategies = append(a.Strategies, strategy)
	return nil
}

func (a *Account) FindStrategy(strategyName string) (*Strategy, error) {
	for _, s := range a.Strategies {
		if strategyName == s.Name {
			return s, nil
		}
	}

	return nil, fmt.Errorf("Account.FindStrategy: could not find strategy with name %v", strategyName)
}

/*
PlaceOrderClose closes a percentage of all trades at the specified price level
params:
strategyName is the name of the strategy to close
priceLevelIndex is an integer between zero and the number of price levels that we wish to close
closePercent is the percentage of trades to close. Trades will be closed either by FIFO or LIFO
*/
func (a *Account) PlaceOrderClose(priceLevel *PriceLevel, closePercentage float64, reason string) (CloseTradesRequestV1, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	closeMethod := FIFO

	closePercent := ClosePercent(closePercentage)
	if err := closePercent.Validate(); err != nil {
		return nil, fmt.Errorf("PlaceOrderClose: %w", err)
	}

	_, v, _ := priceLevel.Trades.GetTradeStatsItems()
	vol := math.Abs(float64(v))
	targetVolume := vol * float64(closePercent)

	var closeTradesRequests []*CloseTradeRequestV1
	switch closeMethod {
	case FIFO:
		reducedVolume := 0.0
		for _, tr := range *priceLevel.Trades {
			remainingCloseVolume := vol - reducedVolume

			if remainingCloseVolume >= targetVolume {
				break
			}

			var _closePercentage float64
			if math.Abs(tr.RequestedVolume) <= remainingCloseVolume {
				_closePercentage = 1
			} else {
				_closePercentage = remainingCloseVolume
			}

			closeTradesRequests = append(closeTradesRequests, &CloseTradeRequestV1{
				Trade:  tr,
				Reason: reason,
				Volume: tr.ExecutedVolume * _closePercentage,
			})

			reducedVolume += math.Abs(tr.RequestedVolume) * _closePercentage
		}
	case LIFO:
		panic("closeMethod LIFO: not yet implemented")
	default:
		panic("closeMethod not yet implemented")
	}

	return closeTradesRequests, nil
}

func NewAccount(name string, balance float64, datafeed *Datafeed, env string) (*Account, error) {
	switch datafeed.Name {
	case CoinbaseDatafeed:
	case IBDatafeed:
	case ManualDatafeed:
		if env == "PRODUCTION" {
			log.Fatalf("cannot use manual datafeed in production")
		}
	default:
		log.Fatalf("unknown datafeedName: %v", datafeed.Name)
	}

	return &Account{
		Name:       name,
		Strategies: make([]*Strategy, 0),
		Balance:    balance,
		Datafeed:   datafeed,
	}, nil
}
