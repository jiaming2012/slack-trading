package integrationtesting

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func validateOrders(ctx context.Context, t *testing.T, playgroundClient playground.PlaygroundService, playgroundId string, orders []*playground.Order) {
	// Fetch orders
	accountsResp, err := playgroundClient.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: playgroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)

	require.Len(t, accountsResp.Orders, 3)

	// Inspect the open order
	require.Equal(t, orders[0].Id, accountsResp.Orders[0].Id)
	require.Len(t, accountsResp.Orders[0].ClosedBy, 2)

	// Inspect the 1st partial close order
	require.Equal(t, orders[1].Id, accountsResp.Orders[1].Id)
	require.Len(t, accountsResp.Orders[1].ClosedBy, 0)
	require.Len(t, accountsResp.Orders[1].Closes, 1)
	require.Equal(t, orders[0].Id, accountsResp.Orders[1].Closes[0].Id)

	// Inspect the 2nd partial close order
	require.Equal(t, orders[2].Id, accountsResp.Orders[2].Id)
	require.Len(t, accountsResp.Orders[2].ClosedBy, 0)
	require.Len(t, accountsResp.Orders[2].Closes, 1)
	require.Equal(t, orders[0].Id, accountsResp.Orders[2].Closes[0].Id)
}

func TestWithPostgres(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)

	// Start main app container
	playgroundClient := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	fetchPlaygrounds := func() ([]*playground.PlaygroundSession, error) {
		resp, err := playgroundClient.GetPlaygrounds(ctx, &playground.GetPlaygroundsRequest{})
		if err != nil {
			return nil, err
		}

		return resp.Playgrounds, nil
	}

	allPlaygrounds, err := fetchPlaygrounds()
	require.NoError(t, err)

	require.Len(t, allPlaygrounds, 0)

	playgroundResp, err := playgroundClient.CreatePlayground(ctx, &playground.CreatePolygonPlaygroundRequest{
		Balance:   10000,
		StartDate: "2021-01-04",
		StopDate:  "2021-01-05",
		Repositories: []*playground.Repository{
			{
				Symbol:             "AAPL",
				TimespanMultiplier: 1,
				TimespanUnit:       "minute",
				Indicators:         []string{},
				HistoryInDays:      0,
			},
		},
		Environment: "simulator",
	})

	require.NoError(t, err)

	require.Greater(t, len(playgroundResp.Id), 0)

	allPlaygrounds, err = fetchPlaygrounds()
	require.NoError(t, err)

	require.Len(t, allPlaygrounds, 1)

	// Place open order
	order1, err := playgroundClient.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   playgroundResp.Id,
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       10,
		RequestedPrice: 100,
		Side:           "buy",
		Type:           "market",
		Duration:       "day",
	})

	require.NoError(t, err)

	// Send tick
	_, err = playgroundClient.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: playgroundResp.Id,
		Seconds:      60,
		RequestId:    "postgres_test:1",
	})

	require.NoError(t, err)

	// Place 1st partial close order
	order2, err := playgroundClient.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   playgroundResp.Id,
		Symbol:         "AAPL",
		RequestedPrice: 100,
		AssetClass:     "equity",
		Quantity:       5,
		Side:           "sell",
		Type:           "market",
		Duration:       "day",
	})

	require.NoError(t, err)

	// Place 2nd partial close order
	order3, err := playgroundClient.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   playgroundResp.Id,
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       5,
		RequestedPrice: 100,
		Side:           "sell",
		Type:           "market",
		Duration:       "day",
	})

	require.NoError(t, err)

	// Send tick
	_, err = playgroundClient.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: playgroundResp.Id,
		Seconds:      60,
		RequestId:    "postgres_test:2",
	})

	require.NoError(t, err)

	// Fetch and validate orders
	validateOrders(ctx, t, playgroundClient, playgroundResp.Id, []*playground.Order{order1, order2, order3})

	// Save the playground
	_, err = playgroundClient.SavePlayground(ctx, &playground.SavePlaygroundRequest{
		PlaygroundId: playgroundResp.Id,
	})

	require.NoError(t, err)

	// Restart the app container
	playgroundClient = createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	allPlaygrounds, err = fetchPlaygrounds()
	require.NoError(t, err)

	require.Len(t, allPlaygrounds, 1)

	// Fetch and validate orders
	validateOrders(ctx, t, playgroundClient, playgroundResp.Id, []*playground.Order{order1, order2, order3})
}
