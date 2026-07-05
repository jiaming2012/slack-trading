package integrationtesting

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/playground"
)

func chooseSpreadContracts(contracts []*playground.OptionLadderContract) (c1 *playground.OptionLadderContract, c2 *playground.OptionLadderContract) {
	// for testing purpose, just return the first two call contracts
	for _, contract := range contracts {
		if contract.Type == "call" {
			if c1 == nil {
				c1 = contract
				continue
			}

			c2 = contract
			break
		}
	}

	return
}

func TestTradierPlaceSpread(t *testing.T) {
	t.Run("Place a spread order", func(t *testing.T) {
		ctx := context.Background()
		goEnv := "test"

		projectsDir, networkName := setupDatabases(t, ctx, goEnv)

		// Start main app container
		client := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

		pg, err := client.CreateLivePlayground(ctx, &playground.CreateLivePlaygroundRequest{
			Balance:     10000.0,
			Broker:      "tradier",
			AccountType: "mock",
			Repositories: []*playground.Repository{
				{
					Symbol:             "AAPL",
					TimespanMultiplier: 1,
					TimespanUnit:       "hour",
					Indicators:         []string{},
					HistoryInDays:      10,
				},
			},
			Environment: "live",
		})

		_ = pg

		require.NoError(t, err)

		resp, err := client.GetOptionsLadder(ctx, &playground.GetOptionsLadderRequest{
			PlaygroundId:              pg.Id,
			StockSymbol:               "AAPL",
			MaxNoOfStrikes:            3,
			MinDistanceBetweenStrikes: 1,
			ExpirationInDays:          []int32{7},
			MaxTickAgeInMinutes:       1440,
		})

		require.NoError(t, err)

		c1, c2 := chooseSpreadContracts(resp.Contracts)
		require.NotNil(t, c1)
		require.NotNil(t, c2)

		order, err := client.PlaceMultiLegOrder(ctx, &playground.PlaceMultiLegOrderRequest{
			PlaygroundId: pg.Id,
			Legs: []*playground.MultiLegOrderLeg{
				{
					Symbol:     c1.Symbol,
					Quantity:   1,
					Side:       "buy_to_open",
					AssetClass: string(models.OrderRecordClassOption),
					Tag:        "test_spread_order_1",
				},
				{
					Symbol:     c2.Symbol,
					Quantity:   1,
					Side:       "sell_to_open",
					AssetClass: string(models.OrderRecordClassOption),
					Tag:        "test_spread_order_2",
				},
			},
			Type:     "market",
			Duration: "gtc",
		})

		require.NoError(t, err)
		require.NotNil(t, order)
	})
}
