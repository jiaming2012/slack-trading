package eventservices

import (
	"slack-trading/src/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUpdateConditions(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	strategyName := "testStrategy"
	signalName := "testSignal"
	symbol := "testSymbol"
	priceLevels := []*models.PriceLevel{
		{
			Price: 1.0,
		},
		{
			Price:             2.0,
			MaxNoOfTrades:     2,
			AllocationPercent: 0.5,
			StopLoss:          3.5,
		},
		{
			Price:             3.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
			StopLoss:          4.0,
		},
	}

	t.Run("0 entry conditions", func(t *testing.T) {
		account, err := models.NewAccount("test account", 1000, nil)
		assert.NoError(t, err)

		accounts := []models.Account{*account}
		signalRequest := models.NewSignalRequest(id, signalName)

		entryConditionsSatisfied := UpdateEntryConditions(accounts, signalRequest)
		assert.Len(t, entryConditionsSatisfied, 0)
	})

	t.Run("1 entry condition", func(t *testing.T) {
		account, err := models.NewAccount("test account", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(strategyName, symbol, models.Down, 100, priceLevels, account)
		assert.NoError(t, err)

		entrySignalName := "entry1"
		entryCondition := models.SignalV2{Name: entrySignalName}
		exitCondition := models.SignalV2{Name: "exit1"}

		strategy.AddEntryCondition(&entryCondition, &exitCondition)
		account.AddStrategy(*strategy)
		accounts := []models.Account{*account}

		entryConditionsSatisfied := UpdateEntryConditions(accounts, models.NewSignalRequest(id, entrySignalName))
		assert.Len(t, entryConditionsSatisfied, 1)
		assert.Equal(t, account, entryConditionsSatisfied[0].Account)
		assert.Equal(t, strategy, entryConditionsSatisfied[0].Strategy)
	})

	t.Run("missed entry condition", func(t *testing.T) {
		account, err := models.NewAccount("test account", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(strategyName, symbol, models.Down, 100, priceLevels, nil)
		assert.NoError(t, err)

		entrySignalName := "entry1"
		otherSignalName := "entry2"
		entryCondition := models.SignalV2{Name: entrySignalName}
		exitCondition := models.SignalV2{Name: "exit1"}

		strategy.AddEntryCondition(&entryCondition, &exitCondition)
		account.AddStrategy(*strategy)
		accounts := []models.Account{*account}

		entryConditionsSatisfied := UpdateEntryConditions(accounts, models.NewSignalRequest(id, otherSignalName))
		assert.Len(t, entryConditionsSatisfied, 0)
	})

	t.Run("2 entry conditions", func(t *testing.T) {
		account, err := models.NewAccount("test account", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(strategyName, symbol, models.Down, 100, priceLevels, account)
		assert.NoError(t, err)

		entryCondition1 := models.SignalV2{Name: "entry1"}
		entryCondition2 := models.SignalV2{Name: "entry2"}
		exitCondition := models.SignalV2{Name: "exit1"}

		strategy.AddEntryCondition(&entryCondition1, &exitCondition)
		strategy.AddEntryCondition(&entryCondition2, &exitCondition)
		account.AddStrategy(*strategy)
		accounts := []models.Account{*account}

		entryConditionsSatisfied := UpdateEntryConditions(accounts, models.NewSignalRequest(id, entryCondition1.Name))
		assert.Len(t, entryConditionsSatisfied, 0)
		entryConditionsSatisfied = UpdateEntryConditions(accounts, models.NewSignalRequest(id, entryCondition2.Name))
		assert.Len(t, entryConditionsSatisfied, 1)
		assert.Equal(t, strategy, entryConditionsSatisfied[0].Strategy)
	})

	t.Run("entry condition not satisfied when exit condition is satisfied", func(t *testing.T) {
		account, err := models.NewAccount("test account", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(strategyName, symbol, models.Down, 100, priceLevels, account)
		assert.NoError(t, err)

		entryCondition1 := models.SignalV2{Name: "entry1"}
		entryCondition2 := models.SignalV2{Name: "entry2"}
		exitCondition1 := models.SignalV2{Name: "exit1"}
		exitCondition2 := models.SignalV2{Name: "exit2"}

		strategy.AddEntryCondition(&entryCondition1, &exitCondition1)
		strategy.AddEntryCondition(&entryCondition2, &exitCondition2)
		account.AddStrategy(*strategy)
		accounts := []models.Account{*account}

		entryConditionsSatisfied := UpdateEntryConditions(accounts, models.NewSignalRequest(id, entryCondition1.Name))
		assert.Len(t, entryConditionsSatisfied, 0)

		entryConditionsSatisfied = UpdateEntryConditions(accounts, models.NewSignalRequest(id, exitCondition1.Name))
		assert.Len(t, entryConditionsSatisfied, 0)

		entryConditionsSatisfied = UpdateEntryConditions(accounts, models.NewSignalRequest(id, entryCondition2.Name))
		assert.Len(t, entryConditionsSatisfied, 0)

		entryConditionsSatisfied = UpdateEntryConditions(accounts, models.NewSignalRequest(id, entryCondition1.Name))
		assert.Len(t, entryConditionsSatisfied, 1)

		entryConditionsSatisfied = UpdateEntryConditions(accounts, models.NewSignalRequest(id, exitCondition2.Name))
		assert.Len(t, entryConditionsSatisfied, 0)
	})
}

func TestGetStatsDownDirection(t *testing.T) {
	name := "Test Account"
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	direction := models.Down
	symbol := "BTCUSD"
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	tf := new(int)
	*tf = 5

	priceLevels := []*models.PriceLevel{
		{
			Price: 1.0,
		},
		{
			Price:             2.0,
			MaxNoOfTrades:     2,
			AllocationPercent: 0.5,
			StopLoss:          3.5,
		},
		{
			Price:             3.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
			StopLoss:          4.0,
		},
	}

	t.Run("test using an up and down strategy", func(t *testing.T) {
		//panic("implement the test")
	})

	t.Run("open trades adjust after a 50% partial close", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(name, symbol, direction, 100, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(*strategy)
		assert.NoError(t, err)

		stats, err := GetStats(id, account, &models.Tick{Bid: 1.5, Ask: 1.5})
		assert.NoError(t, err)

		// no trades open
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Equal(t, name, stats.Strategies[0].StrategyName)
		assert.Equal(t, models.Volume(0), stats.Strategies[0].Stats.Volume)
		assert.Equal(t, 3, len(stats.Strategies[0].OpenTradeLevels))
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[0].Trades))
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[1].Trades))
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[2].Trades))

		// open two trades
		requestedPrice := 1.5
		priceLevelIndex := 1

		tr1, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.3, Ask: 1.3})
		assert.NoError(t, err)

		strategyVolume := stats.Strategies[0].Stats.Volume
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Less(t, strategyVolume, 0.0)
		assert.Equal(t, 0.0, stats.Strategies[0].Stats.RealizedPL)
		assert.Greater(t, stats.Strategies[0].Stats.FloatingPL, 0.0)
		assert.Equal(t, models.Vwap(requestedPrice), stats.Strategies[0].Stats.Vwap)
		assert.Equal(t, 2, len(stats.Strategies[0].OpenTradeLevels[priceLevelIndex].Trades))

		// partial close
		tr3, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, priceLevelIndex, 0.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		assert.Equal(t, strategyVolume/2.0, stats.Strategies[0].Stats.Volume)
		assert.Less(t, stats.Strategies[0].Stats.FloatingPL, models.FloatingPL(0))
		assert.Less(t, stats.Strategies[0].Stats.RealizedPL, models.RealizedPL(0))
		assert.Equal(t, 1, len(stats.Strategies[0].OpenTradeLevels[priceLevelIndex].Trades))
		assert.Equal(t, tr2.ID, stats.Strategies[0].OpenTradeLevels[priceLevelIndex].Trades[0].ID)
	})

	t.Run("open trades adjust after a full close", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(name, symbol, direction, 100, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(*strategy)
		assert.NoError(t, err)

		stats, err := GetStats(id, account, &models.Tick{Bid: 1.5, Ask: 1.5})
		assert.NoError(t, err)

		// open three trades
		requestedPrice := 2.5

		tr1, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		tr3, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.3, Ask: 1.3})
		assert.NoError(t, err)

		strategyVolume := stats.Strategies[0].Stats.Volume
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Less(t, strategyVolume, 0.0)

		tradesIndex := 2
		assert.Equal(t, 3, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// partial close
		tr4, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr4)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		assert.Equal(t, models.Volume(0), stats.Strategies[0].Stats.Volume)
		assert.Equal(t, 0.0, stats.Strategies[0].Stats.FloatingPL)
		assert.Greater(t, stats.Strategies[0].Stats.RealizedPL, 0.0)
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))
	})

	t.Run("open trades adjust after a full close via two partial closes", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(name, symbol, direction, 100, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(*strategy)
		assert.NoError(t, err)

		stats, err := GetStats(id, account, &models.Tick{Bid: 1.5, Ask: 1.5})
		assert.NoError(t, err)

		// open two trades
		requestedPrice := 2.5

		tr1, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.3, Ask: 1.3})
		assert.NoError(t, err)

		strategyVolumeV1 := stats.Strategies[0].Stats.Volume
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Less(t, strategyVolumeV1, 0.0)

		tradesIndex := 2
		assert.Equal(t, 2, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// partial close: 75%
		closePerc := 0.75
		tr3, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, closePerc)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		strategyVolumeV2 := float64(strategyVolumeV1) * (1 - closePerc)
		assert.InEpsilon(t, strategyVolumeV2, float64(stats.Strategies[0].Stats.Volume), models.SmallRoundingError)
		assert.Equal(t, 1, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// partial close: 50%
		closePerc = 0.5
		tr4, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, closePerc)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr4)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		strategyVolumeV3 := strategyVolumeV2 * (1 - closePerc)
		assert.InEpsilon(t, strategyVolumeV3, float64(stats.Strategies[0].Stats.Volume), models.SmallRoundingError)
		assert.Equal(t, 1, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// close: 100%
		tr5, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr5)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		assert.Equal(t, models.Volume(0), stats.Strategies[0].Stats.Volume)
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))
	})
}

func TestGetStatsUpDirection(t *testing.T) {
	name := "Test Account"
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	direction := models.Up
	symbol := "BTCUSD"
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	tf := new(int)
	*tf = 5
	priceLevels := []*models.PriceLevel{
		{
			Price:             1.0,
			MaxNoOfTrades:     2,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             2.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             3.0,
			AllocationPercent: 0,
		},
	}

	t.Run("open trades adjust after a 50% partial close", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(name, symbol, direction, 100, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(*strategy)
		assert.NoError(t, err)

		stats, err := GetStats(id, account, &models.Tick{Bid: 1.5, Ask: 1.5})
		assert.NoError(t, err)

		// no trades open
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Equal(t, name, stats.Strategies[0].StrategyName)
		assert.Equal(t, models.Volume(0), stats.Strategies[0].Stats.Volume)
		assert.Equal(t, 3, len(stats.Strategies[0].OpenTradeLevels))
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[0].Trades))
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[1].Trades))
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[2].Trades))

		// open two trades
		requestedPrice := 1.5

		tr1, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.3, Ask: 1.3})
		assert.NoError(t, err)

		strategyVolume := stats.Strategies[0].Stats.Volume
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Greater(t, strategyVolume, 0.0)
		assert.Equal(t, 0.0, stats.Strategies[0].Stats.RealizedPL)
		assert.Less(t, stats.Strategies[0].Stats.FloatingPL, 0.0)
		assert.Equal(t, models.Vwap(requestedPrice), stats.Strategies[0].Stats.Vwap)
		assert.Equal(t, 2, len(stats.Strategies[0].OpenTradeLevels[0].Trades))

		// partial close
		tr3, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, 0, 0.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		assert.Equal(t, strategyVolume/2.0, stats.Strategies[0].Stats.Volume)
		assert.Greater(t, stats.Strategies[0].Stats.FloatingPL, models.FloatingPL(0))
		assert.Greater(t, stats.Strategies[0].Stats.RealizedPL, models.RealizedPL(0))
		assert.Equal(t, 1, len(stats.Strategies[0].OpenTradeLevels[0].Trades))
		assert.Equal(t, tr2.ID, stats.Strategies[0].OpenTradeLevels[0].Trades[0].ID)
	})

	t.Run("open trades adjust after a full close", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(name, symbol, direction, 100, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(*strategy)
		assert.NoError(t, err)

		stats, err := GetStats(id, account, &models.Tick{Bid: 1.5, Ask: 1.5})
		assert.NoError(t, err)

		// open three trades
		requestedPrice := 2.5

		tr1, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		tr3, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.3, Ask: 1.3})
		assert.NoError(t, err)

		strategyVolume := stats.Strategies[0].Stats.Volume
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Greater(t, strategyVolume, 0.0)

		tradesIndex := 1
		assert.Equal(t, 3, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// partial close
		tr4, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr4)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		assert.Equal(t, models.Volume(0), stats.Strategies[0].Stats.Volume)
		assert.Equal(t, 0.0, stats.Strategies[0].Stats.FloatingPL)
		assert.Less(t, stats.Strategies[0].Stats.RealizedPL, 0.0)
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))
	})

	t.Run("open trades adjust after a full close via two partial closes", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(name, symbol, direction, 100, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(*strategy)
		assert.NoError(t, err)

		stats, err := GetStats(id, account, &models.Tick{Bid: 1.5, Ask: 1.5})
		assert.NoError(t, err)

		// open two trades
		requestedPrice := 2.5

		tr1, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := strategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.3, Ask: 1.3})
		assert.NoError(t, err)

		strategyVolumeV1 := stats.Strategies[0].Stats.Volume
		assert.Equal(t, 1, len(stats.Strategies))
		assert.Greater(t, strategyVolumeV1, 0.0)

		tradesIndex := 1
		assert.Equal(t, 2, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// partial close: 75%
		closePerc := 0.75
		tr3, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, closePerc)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		strategyVolumeV2 := float64(strategyVolumeV1) * (1 - closePerc)
		assert.InEpsilon(t, strategyVolumeV2, float64(stats.Strategies[0].Stats.Volume), models.SmallRoundingError)
		assert.Equal(t, 1, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// partial close: 50%
		closePerc = 0.5
		tr4, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, closePerc)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr4)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		strategyVolumeV3 := strategyVolumeV2 * (1 - closePerc)
		assert.InEpsilon(t, strategyVolumeV3, float64(stats.Strategies[0].Stats.Volume), models.SmallRoundingError)
		assert.Equal(t, 1, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))

		// close: 100%
		tr5, _, err := strategy.NewCloseTrades(id, tf, ts, 1.8, tradesIndex, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr5)
		assert.NoError(t, err)

		stats, err = GetStats(id, account, &models.Tick{Bid: 1.8, Ask: 1.8})
		assert.NoError(t, err)

		assert.Equal(t, models.Volume(0), stats.Strategies[0].Stats.Volume)
		assert.Equal(t, 0, len(stats.Strategies[0].OpenTradeLevels[tradesIndex].Trades))
	})
}

func TestFetchTrades(t *testing.T) {
	name := "Test Account"
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	direction := models.Up
	symbol := "BTCUSD"
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	tf := new(int)
	*tf = 5
	priceLevels := []*models.PriceLevel{
		{
			Price:             1.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             2.0,
			MaxNoOfTrades:     1,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             3.0,
			AllocationPercent: 0,
		},
	}

	t.Run("Fetch trades is nil when no trades have been placed", func(t *testing.T) {
		account, err := models.NewAccount("test", 1000, nil)
		assert.NoError(t, err)
		result := FetchTrades(id, account)
		assert.NotNil(t, result)
		assert.Nil(t, result.Trades)
	})

	t.Run("one buy trade", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, nil)
		assert.NoError(t, err)

		strategy, err := models.NewStrategyDeprecated(name, symbol, direction, 100, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(*strategy)
		assert.NoError(t, err)

		tr, _, err := strategy.NewOpenTrade(id, tf, ts, 1.5)
		assert.NoError(t, err)

		_, err = strategy.AutoExecuteTrade(tr)
		assert.NoError(t, err)

		result := FetchTrades(id, account)
		assert.NotNil(t, result)
		assert.Equal(t, id, result.GetRequestID())
		assert.Equal(t, len(priceLevels), len(result.Trades))
		assert.Equal(t, len(result.Trades[0].Trades), 1)
		assert.Equal(t, len(result.Trades[1].Trades), 0)
		assert.Equal(t, len(result.Trades[2].Trades), 0)

		assert.NotNil(t, result.Trades)
	})
}
