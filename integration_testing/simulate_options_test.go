package integrationtesting

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestSimulateOptions(t *testing.T) {
	t.Run("Sell OTM call option - expire worthless", func(t *testing.T) {
		ctx := context.Background()
		goEnv := "test"

		projectsDir, networkName := setupDatabases(t, ctx, goEnv)

		// Start main app container
		client := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

		pg, err := client.CreatePlayground(ctx, &playground.CreatePolygonPlaygroundRequest{
			Balance:   10000.0,
			StartDate: "2025-09-03",
			StopDate:  "2025-09-06",
			Repositories: []*playground.Repository{
				{
					Symbol:             "AAPL",
					TimespanMultiplier: 1,
					TimespanUnit:       "hour",
					Indicators:         []string{},
					HistoryInDays:      10,
				},
			},
			Environment: "simulator",
		})

		require.NoError(t, err)

		order, err := client.PlaceOrder(ctx, &playground.PlaceOrderRequest{
			PlaygroundId:   pg.Id,
			Symbol:         "O:AAPL250905C00230000",
			AssetClass:     "option",
			Quantity:       1,
			Side:           "sell_to_open",
			Type:           "market",
			Duration:       "day",
			Tag:            "",
			RequestedPrice: 1.0,
			IsAdjustment:   false,
		})

		require.NoError(t, err)

		require.NotNil(t, order)

		// tick
		tickDelta, err := client.NextTick(ctx, &playground.NextTickRequest{
			PlaygroundId: pg.Id,
			Seconds:      0.0,
			IsPreview:    false,
			RequestId:    fmt.Sprintf("test-request-id-%s", uuid.New().String()),
		})

		require.NoError(t, err)

		require.Len(t, tickDelta.NewTrades, 1)
		require.Equal(t, -1.0, tickDelta.NewTrades[0].Quantity)
		require.Greater(t, tickDelta.NewTrades[0].Price, 0.0)

		require.Fail(t, "finish the test case")
	})

	// t.Run("Buy an option from the ladder", func(t *testing.T) {
	// 	ctx := context.Background()
	// 	goEnv := "test"

	// 	projectsDir, networkName := setupDatabases(t, ctx, goEnv)

	// 	// Start main app container
	// 	client := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	// 	pg, err := client.CreatePlayground(ctx, &playground.CreatePolygonPlaygroundRequest{
	// 		Balance:   10000.0,
	// 		StartDate: "2025-08-01",
	// 		StopDate:  "2025-08-10",
	// 		Repositories: []*playground.Repository{
	// 			{
	// 				Symbol:             "XYZ",
	// 				TimespanMultiplier: 1,
	// 				TimespanUnit:       "hour",
	// 				Indicators:         []string{},
	// 				HistoryInDays:      10,
	// 			},
	// 		},
	// 		Environment: "simulator",
	// 	})

	// 	require.NoError(t, err)

	// 	ladderResp, err := client.GetOptionsLadder(ctx, &playground.GetOptionsLadderRequest{
	// 		PlaygroundId:              pg.Id,
	// 		StockSymbol:               "XYZ",
	// 		MaxNoOfStrikes:            3,
	// 		MinDistanceBetweenStrikes: 1,
	// 		ExpirationInDays:          []int32{3},
	// 	})

	// 	require.NoError(t, err)

	// 	require.Greater(t, len(ladderResp.Contracts), 0)

	// 	order, err := client.PlaceOrder(ctx, &playground.PlaceOrderRequest{
	// 		PlaygroundId:   pg.Id,
	// 		Symbol:         ladderResp.Contracts[0].Symbol,
	// 		AssetClass:     "option",
	// 		Quantity:       1,
	// 		Side:           "sell_to_open",
	// 		Type:           "market",
	// 		Duration:       "day",
	// 		Tag:            "",
	// 		RequestedPrice: 1.0,
	// 		IsAdjustment:   false,
	// 	})

	// 	require.NoError(t, err)

	// 	require.Equal(t, ladderResp.Contracts[0].Symbol, order.Symbol)

	// 	tickDelta, err := client.NextTick(ctx, &playground.NextTickRequest{
	// 		PlaygroundId: pg.Id,
	// 		Seconds:      0.0,
	// 		IsPreview:    false,
	// 		RequestId:    fmt.Sprintf("test-request-id-%s", uuid.New().String()),
	// 	})

	// 	require.NoError(t, err)

	// 	fmt.Printf("Tick Delta: %+v\n", tickDelta)
	// })
}
