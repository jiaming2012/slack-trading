package integrationtests

import (
	"fmt"
	"testing"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
	"github.com/stretchr/testify/require"
)

func TestPolygonClient(t *testing.T) {
	projectsDir := "/Users/jamal/projects"
	goEnv := "development"

	err := utils.InitEnvironmentVariables(projectsDir, goEnv)
	require.NoError(t, err)

	polygonApiKey, err := utils.GetEnv("POLYGON_API_KEY")
	require.NoError(t, err)

	t.Run("fetch candles", func(t *testing.T) {

		polygonClient := eventservices.NewPolygonTickDataMachine(polygonApiKey)

		tz, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)

		ts := eventmodels.PolygonTimespan{
			Multiplier: 30,
			Unit:       eventmodels.PolygonTimespanUnitMinute,
		}

		ticker := eventmodels.NewStockSymbol("AAPL")

		start := time.Date(2025, 1, 29, 22, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 29, 23, 0, 0, 0, time.UTC)

		candles, err := polygonClient.FetchAggregateBarsWithDates(ticker, ts, start, end, tz)
		require.NoError(t, err)

		fmt.Printf("+%v\n", candles)

		require.Fail(t, "finish the test")
	})
}
