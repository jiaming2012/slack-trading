package eventconsumers

import (
	"slack-trading/src/models"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createAccountFixtures(accountName string, symbol string, direction models.Direction, strategyName string, balance float64, priceLevels []*models.PriceLevel, datafeed *models.Datafeed) ([]*models.Account, error) {
	accountFixture, err := models.NewAccount(accountName, balance, datafeed)
	if err != nil {
		return nil, err
	}

	trendlineBreakStrategyFixture, err := models.NewStrategyDeprecated(strategyName, symbol, direction, balance, priceLevels, accountFixture)
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
		//datafeed := models.NewDatafeed("test")
		//datafeed.Update(models.Tick{Bid: 0, Ask: 0})
		c := NewAccountWorkerClient(&wg)
		strategy, _, err := c.checkTradeCloseParameters()

		assert.NoError(t, err)
		assert.Nil(t, strategy)
	})

	t.Run("stop out return strategy to be closed", func(t *testing.T) {
		//wg := sync.WaitGroup{}
		symbol := "BTC-USD"
		direction := models.Direction("up")
		accountName := "playground"
		strategyName := "trendline-break"
		datafeed := models.NewDatafeed("test")

		accounts, err := createAccountFixtures(accountName, symbol, direction, strategyName, balance, priceLevels, datafeed)
		assert.NoError(t, err)
		assert.Len(t, accounts, 1)

		//c := NewAccountWorkerClientFromFixtures(&wg, accounts)

	})
}
