package workers

// Tests for the feed-health heartbeat observation in the live candle-ingestion
// path (wire-anomaly-guard-feeds): observeFeedHeartbeat — called by
// updateLiveRepos after AppendBars — must record a wall-clock heartbeat per
// asset class when new bars were appended for a realtime Playground, and must
// stay inert for simulation repos, zero-bar updates, and an unwired monitor.
// (The full updateLiveRepos path shells out to the conda indicator pipeline in
// AppendBars, so the observation seam is what unit tests can pin headlessly.)

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/feedhealth"
	"github.com/jiaming2012/slack-trading/src/go/models"
)

func newHeartbeatTestWorker(t *testing.T) (*TradierApiWorker, *feedhealth.HeartbeatMonitor) {
	t.Helper()
	wg := sync.WaitGroup{}
	worker := NewTradierApiWorker(&wg, "", "", nil, nil, "", nil, nil)
	monitor := feedhealth.NewHeartbeatMonitor(feedhealth.RealClock{}, feedhealth.ThresholdConfig{})
	worker.SetFeedHeartbeatMonitor(monitor)
	return worker, monitor
}

func TestObserveFeedHeartbeat_NewBarsForRealtimePlaygroundRecordHeartbeat(t *testing.T) {
	worker, monitor := newHeartbeatTestWorker(t)

	before := time.Now()
	worker.observeFeedHeartbeat(models.NewStockSymbol("AAPL"), 3, true)

	observed, ok := monitor.LastObserved("equity")
	require.True(t, ok, "appending new live bars must record a feed-health heartbeat for the repo's asset class")
	require.False(t, observed.Before(before), "the heartbeat must be recorded at wall-clock RECEIPT time, not the bar timestamp")

	// Option repos heartbeat under their own asset class.
	worker.observeFeedHeartbeat(models.OptionSymbol("AAPL250117C00150000"), 1, true)
	_, ok = monitor.LastObserved("option")
	require.True(t, ok)
}

func TestObserveFeedHeartbeat_NonRealtimePlaygroundDoesNotFeed(t *testing.T) {
	worker, monitor := newHeartbeatTestWorker(t)

	worker.observeFeedHeartbeat(models.NewStockSymbol("AAPL"), 3, false)

	_, ok := monitor.LastObserved("equity")
	require.False(t, ok, "a non-realtime playground's repo update must not feed the heartbeat monitor")
}

func TestObserveFeedHeartbeat_ZeroNewBarsDoesNotFeed(t *testing.T) {
	worker, monitor := newHeartbeatTestWorker(t)

	worker.observeFeedHeartbeat(models.NewStockSymbol("AAPL"), 0, true)

	_, ok := monitor.LastObserved("equity")
	require.False(t, ok, "a no-new-candles update is not a Tick and must not refresh the heartbeat")
}

func TestObserveFeedHeartbeat_NilMonitorIsInert(t *testing.T) {
	wg := sync.WaitGroup{}
	worker := NewTradierApiWorker(&wg, "", "", nil, nil, "", nil, nil)

	require.NotPanics(t, func() {
		worker.observeFeedHeartbeat(models.NewStockSymbol("AAPL"), 3, true)
	})
}

func TestAssetClassForInstrument(t *testing.T) {
	require.Equal(t, "equity", assetClassForInstrument(models.NewStockSymbol("AAPL")))
	require.Equal(t, "option", assetClassForInstrument(models.OptionSymbol("AAPL250117C00150000")))
}
