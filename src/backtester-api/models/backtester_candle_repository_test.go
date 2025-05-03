package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func TestSymbol(t *testing.T) {
	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	require.NoError(t, err)

	goEnv := "test"

	err = utils.InitEnvironmentVariables(projectsDir, goEnv)
	require.NoError(t, err)

	t.Run("returns the symbol", func(t *testing.T) {
		symbol := eventmodels.StockSymbol("AAPL")
		period := time.Minute
		source := eventmodels.CandleRepositorySource{
			Type: "polygon",
		}

		repo, err := NewCandleRepository(symbol, period, nil, []string{}, nil, 0, source)

		require.NoError(t, err)

		require.Equal(t, symbol, repo.GetSymbol())
	})
}

func TestNext(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	period := time.Minute
	source := eventmodels.CandleRepositorySource{
		Type: "polygon",
	}

	candles := []*eventmodels.PolygonAggregateBarV2{
		{
			Timestamp: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Timestamp: time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC),
		},
		{
			Timestamp: time.Date(2021, 1, 1, 0, 2, 0, 0, time.UTC),
		},
	}

	t.Run("returns the current candle", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)

		require.NoError(t, err)

		tstamp := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		err = repo.SetStartingPosition(tstamp, PlaygroundEnvironmentSimulator, nil)

		require.NoError(t, err)

		_, err = repo.Update(tstamp)

		require.NoError(t, err)

		candle, err := repo.GetCurrentCandle()

		require.NoError(t, err)

		require.Equal(t, tstamp, candle.Timestamp)
	})

	t.Run("returns the next candle", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)

		require.NoError(t, err)

		tstamp := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)

		err = repo.SetStartingPosition(tstamp, PlaygroundEnvironmentSimulator, nil)

		require.NoError(t, err)

		c, err := repo.Update(tstamp)

		require.NoError(t, err)

		require.NotNil(t, c)

		candle, err := repo.GetCurrentCandle()

		require.NoError(t, err)

		require.Equal(t, tstamp, candle.Timestamp)
	})

	t.Run("returns last candle if there are no more candles", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)

		require.NoError(t, err)

		tstamp := time.Date(2021, 1, 1, 0, 3, 0, 0, time.UTC)

		err = repo.SetStartingPosition(tstamp, PlaygroundEnvironmentSimulator, nil)

		require.NoError(t, err)

		_, err = repo.Update(tstamp)

		require.NoError(t, err)

		candle, err := repo.GetCurrentCandle()

		require.NoError(t, err)

		require.Equal(t, candles[len(candles)-1].Timestamp, candle.Timestamp)
	})
}
