package service

import (
	"github.com/stretchr/testify/assert"
	"slack-trading/src/models"
	"testing"
)

func TestTradeService(t *testing.T) {
	newAccount := func() *models.Account {
		account, err := models.NewAccount(10000.0, 0.1, models.PriceLevels{
			Values: []*models.PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        2,
					AllocationPercent: 1,
				},
				{
					Price: 2.0,
				},
			},
		})

		assert.Nil(t, err)

		return account
	}

	t.Run("open and close a buy order", func(t *testing.T) {
		account := newAccount()

		t1, e := PlaceBuy(account, 1.2, 1.0)
		assert.Nil(t, e)
		t1.AutoExecute()

		t2, e := PlaceClose(account, 1.5, 1.0)
		assert.Nil(t, e)
		t2.AutoExecute()
	})

	t.Run("open and close a sell order", func(t *testing.T) {
		account := newAccount()
		p1 := 1.5
		p2 := 1.0

		t1, e := PlaceSell(account, p1, 2.0)
		assert.Nil(t, e)
		t1.AutoExecute()

		vwap, volume, realizedPL := account.GetTrades().Vwap()
		assert.Equal(t, models.Vwap(p1), vwap)
		assert.Greater(t, models.Volume(0), volume)
		assert.Equal(t, models.RealizedPL(0.0), realizedPL)

		t2, e := PlaceClose(account, p2, 1.0)
		assert.Nil(t, e)
		t2.AutoExecute()

		vwap, volume, realizedPL = account.GetTrades().Vwap()
		assert.Equal(t, models.Volume(0), volume)
		assert.Equal(t, models.Vwap(0.0), vwap)
		assert.Greater(t, models.RealizedPL(1.0), (p1-p2)*t1.Volume)
	})
}
