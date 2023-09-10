package eventconsumers

import (
	"github.com/stretchr/testify/assert"
	"slack-trading/src/models"
	"sync"
	"testing"
)

func createAccountFixtures(accountName string, symbol string, direction models.Direction, strategyName string, balance float64, priceLevels []*models.PriceLevel) ([]*models.Account, error) {
	accountFixture, err := models.NewAccount(accountName, balance)
	if err != nil {
		return nil, err
	}

	trendlineBreakStrategyFixture, err := models.NewStrategy(strategyName, symbol, direction, balance, priceLevels)
	if err != nil {
		return nil, err
	}

	if err = accountFixture.AddStrategy(*trendlineBreakStrategyFixture); err != nil {
		return nil, err
	}

	return []*models.Account{
		accountFixture,
	}, nil
}

func TestStopLoss(t *testing.T) {

}

func TestTakeProfit(t *testing.T) {

}

func TestStopOut(t *testing.T) {
	balance := 2000.0
	priceLevels := []*models.PriceLevel{
		{
			Price:             24000.0,
			StopLoss:          20000.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
		},
		{
			Price:             25000.0,
			StopLoss:          20000.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
		},
		{
			Price:             26000.0,
			AllocationPercent: 0,
		},
	}

	t.Run("no stop out when first create", func(t *testing.T) {
		wg := sync.WaitGroup{}
		c := NewAccountWorkerClient(&wg)
		strategy := c.checkForStopOut(models.Tick{
			Bid: 0,
			Ask: 0,
		})

		assert.Nil(t, strategy)
	})

	t.Run("stop out return strategy to be closed", func(t *testing.T) {
		//wg := sync.WaitGroup{}
		symbol := "BTC-USD"
		direction := models.Direction("up")
		accountName := "Playground"
		strategyName := "Trendline Break"

		accounts, err := createAccountFixtures(accountName, symbol, direction, strategyName, balance, priceLevels)
		assert.Nil(t, err)
		assert.Len(t, accounts, 1)

		//c := NewAccountWorkerClientFromFixtures(&wg, accounts)

	})
}
