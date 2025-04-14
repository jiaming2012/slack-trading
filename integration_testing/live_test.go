package integrationtesting

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestLiveAccount(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)

	// Start main app container
	playgroundClient := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	fmt.Printf("Playground client: %v\n", playgroundClient)

	_, err := playgroundClient.CreatePlayground(ctx, &playground.CreatePolygonPlaygroundRequest{
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
		Environment: "live",
	})

	require.NoError(t, err)
}
