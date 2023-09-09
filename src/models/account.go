package models

import (
	"fmt"
	"math"
	"strings"
	"sync"
)

type Account struct {
	Name       string
	Balance    float64
	Strategies []Strategy
	mutex      sync.Mutex
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

			out.WriteString("          Entry Conditions:\n")
			for i, cond := range s.Conditions {
				out.WriteString(fmt.Sprintf("               -> %d: %v\n", i, cond.String()))
			}
		}

		strategies = out.String()
	}

	return fmt.Sprintf("Name: %v\n     Starting Balance: $%.2f, \n     Strategies:\n%s", a.Name, a.Balance, strategies)
}

func (a *Account) GetTrades() *Trades {
	trades := Trades{}

	for _, strategy := range a.Strategies {
		trades.BulkAdd(strategy.GetTrades())
	}

	return &trades
}

func (a *Account) Update(price float64) CloseTradesRequest {
	requests := make([]*CloseTradeRequest, 0)

	for _, trade := range *a.GetTrades() {
		if trade.Side() == TradeTypeBuy {
			if price <= trade.StopLoss {
				requests = append(requests, &CloseTradeRequest{
					Trade:  trade,
					Reason: "SL",
					Volume: trade.ExecutedVolume,
				})
			}
		}

		if trade.Side() == TradeTypeSell {
			if price >= trade.StopLoss {
				requests = append(requests, &CloseTradeRequest{
					Trade:  trade,
					Reason: "SL",
					Volume: trade.ExecutedVolume,
				})
			}
		}
	}

	if len(requests) > 0 {
		return requests
	}

	return nil
}

//func (a *Account) BulkClose(price float64, req BulkCloseRequest) ([]*PriceLevel, error) {
//	if a.PriceLevelsInput != nil {
//		for _, level := range a.PriceLevelsInput.Values {
//			bulkCloseReq := BulkCloseRequest{
//				Items:[]BulkCloseRequestItem{
//					{
//						Level: level,
//						ClosePercent:
//					},
//				},
//			}
//			//level.
//		}
//	}
//}

func (a *Account) getStrategiesBalance() float64 {
	balance := 0.0

	for _, s := range a.Strategies {
		balance += s.Balance
	}

	return balance
}

func (a *Account) AddStrategy(strategy Strategy) error {
	for _, s := range a.Strategies {
		if strategy.Name == s.Name {
			return fmt.Errorf("Account.AddStrategy: strategy name %v must be unique", strategy.Name)
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
			return &s, nil
		}
	}

	return nil, fmt.Errorf("Account.FindStrategy: could not find strategy with name %v", strategyName)
}

func (a *Account) PlaceOpenTradeRequest(strategyName string, currentPrice float64) (*OpenTradeRequest, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	strategy, err := a.FindStrategy(strategyName)
	if err != nil {
		return nil, fmt.Errorf("Account.PlaceOrderOpen: failed to find strategy: %w", err)
	}

	currentPriceLevel := strategy.findPriceLevel(currentPrice)
	if currentPriceLevel == nil {
		return nil, fmt.Errorf("could not find price level at %.2f", currentPrice)
	}

	// todo: refactor PlaceOrder parameters to pass in a trade request
	tradeRequest := OpenTradeRequest{
		Symbol:   strategy.Symbol,
		Type:     strategy.GetTradeType(),
		Price:    currentPrice,
		Strategy: strategy,
		StopLoss: currentPriceLevel.StopLoss,
	}

	return &tradeRequest, nil
}

/*
PlaceOrderClose closes a percentage of all trades at the specified price level
params:
strategyName is the name of the strategy to close
priceLevelIndex is an integer between zero and the number of price levels that we wish to close
closePercent is the percentage of trades to close. Trades will be closed either by FIFO or LIFO
*/
func (a *Account) PlaceOrderClose(priceLevel *PriceLevel, closePercentage float64, reason string) (CloseTradesRequest, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	closeMethod := FIFO

	closePercent := ClosePercent(closePercentage)
	if err := closePercent.Validate(); err != nil {
		return nil, fmt.Errorf("PlaceOrderClose: %w", err)
	}

	_, v, _ := priceLevel.Trades.Vwap()
	vol := math.Abs(float64(v))
	targetVolume := vol * float64(closePercent)

	var closeTradesRequests []*CloseTradeRequest
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

			closeTradesRequests = append(closeTradesRequests, &CloseTradeRequest{
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

//func (a *Account) placeOrder(strategyName string, tradeType TradeType, currentPrice float64, stopLoss float64, closePercent float64) (*PriceLevel, error) {
//
//	var volume float64
//	if tradeType == TradeTypeBuy {
//		if strategyVolume >= 0 {
//			if stopLoss >= currentPrice {
//				return nil, fmt.Errorf("%w: stopLoss of %v is above current price of %v", InvalidStopLossErr, stopLoss, currentPrice)
//			}
//
//			volume = (tradeParams.MaxLoss + float64(realizedPL)) / (currentPrice - stopLoss)
//		} else {
//			// i dont like this. what if closePercent is accidentally zero?
//			if err = ClosePercent(closePercent).Validate(); err != nil {
//				return nil, err
//			}
//			volume = float64(strategyVolume) * closePercent * -1
//		}
//	} else if tradeType == TradeTypeSell {
//		if strategyVolume <= 0 {
//			if stopLoss <= currentPrice {
//				return nil, fmt.Errorf("%w: stopLoss of %v is below current price of %v", InvalidStopLossErr, stopLoss, currentPrice)
//			}
//
//			volume = (tradeParams.MaxLoss + float64(realizedPL)) / (currentPrice - stopLoss)
//		} else {
//			if err = ClosePercent(closePercent).Validate(); err != nil {
//				return nil, err
//			}
//			volume = float64(strategyVolume) * closePercent * -1
//		}
//	} else {
//		return nil, fmt.Errorf("invalid trade type %v", tradeType)
//	}
//
//	newTrade.StopLoss = stopLoss
//	newTrade.RequestedVolume = volume
//
//	if err = newTrade.Validate(false); err != nil {
//		return nil, err
//	}
//
//	tradeParams.PriceLevel.Trades.Add(newTrade)
//
//	return newTrade, nil
//}

func NewAccount(name string, balance float64) (*Account, error) {
	return &Account{
		Name:    name,
		Balance: balance,
	}, nil
}

//func (a *Account) GetBalanceAtLevel(price float64) BalanceLevelStats {
//	return BalanceLevelStats{}
//}
