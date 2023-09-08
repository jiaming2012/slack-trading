package models

import (
	"fmt"
	"math"
	"sync"
)

type Account struct {
	Name       string
	Balance    float64
	Strategies []Strategy
	mutex      sync.Mutex
}

func (a *Account) String() string {
	return fmt.Sprintf("Name: %v, Starting Balance: $%.2f, Strategies: %v", a.Name, a.Balance, a.Strategies)
}

func (a *Account) GetTrades() *Trades {
	trades := Trades{}

	for _, strategy := range a.Strategies {
		trades.BulkAdd(strategy.GetTrades())
	}

	return &trades
}

func (a *Account) Update(price float64) CloseTradesRequest {
	requests := make([]CloseTradeRequest, 0)

	for _, trade := range *a.GetTrades() {
		if trade.Side() == TradeTypeBuy {
			if price <= trade.StopLoss {
				requests = append(requests, CloseTradeRequest{
					Trade:  trade,
					Reason: "SL",
				})
			}
		}

		if trade.Side() == TradeTypeSell {
			if price >= trade.StopLoss {
				requests = append(requests, CloseTradeRequest{
					Trade:  trade,
					Reason: "SL",
				})
			}
		}
	}

	if len(requests) > 0 {
		return requests
	}

	return nil
}

//func (a *Account) BulkClose(price float64, req BulkCloseRequest) ([]*Trade, error) {
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
func (a *Account) PlaceOrderClose(strategyName string, priceLevelIndex int, closePercentage float64, reason string) (CloseTradesRequest, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	closeMethod := FIFO

	closePercent := ClosePercent(closePercentage)
	if err := closePercent.Validate(); err != nil {
		return nil, fmt.Errorf("PlaceOrderClose: %w", err)
	}

	strategy, err := a.FindStrategy(strategyName)
	if err != nil {
		return nil, fmt.Errorf("Account.PlaceOrderClose: failed to find strategy: %w", err)
	}

	if priceLevelIndex < 0 {
		return nil, fmt.Errorf("Account.PlaceOrderClose: priceLevelIndex must be greater than zero")
	}

	if priceLevelIndex >= len(strategy.PriceLevels.Values) {
		return nil, fmt.Errorf("Account.PlaceOrderClose: priceLevelIndex must be less than or equal to the number of price levels (%v) for strategy %v", len(strategy.PriceLevels.Values), strategy.Name)
	}

	priceLevel := strategy.PriceLevels.Values[priceLevelIndex]

	_, v, _ := priceLevel.Trades.Vwap()
	vol := math.Abs(float64(v))
	targetVolume := vol * float64(closePercent)

	var tradeCloseRequests []CloseTradeRequest
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

			tradeCloseRequests = append(tradeCloseRequests, CloseTradeRequest{
				Trade:      tr,
				Reason:     reason,
				Percentage: _closePercentage,
			})

			reducedVolume += math.Abs(tr.RequestedVolume) * _closePercentage
		}
	case LIFO:
		panic("closeMethod LIFO: not yet implemented")
	default:
		panic("closeMethod not yet implemented")
	}

	return tradeCloseRequests, nil
}

//func (a *Account) placeOrder(strategyName string, tradeType TradeType, currentPrice float64, stopLoss float64, closePercent float64) (*Trade, error) {
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
