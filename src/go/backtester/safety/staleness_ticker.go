package safety

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// RunFeedStalenessTicker periodically evaluates the feed-staleness guard
// through the installed registry hook. Evaluation is suppressed while the
// market is closed or while no realtime (Paper/Margin) Playground is loaded,
// so a quiet feed outside trading hours — or on a server running only
// Simulations — can never engage the halt. A failure of the market-calendar
// check itself skips that cycle with a Warn (fail-quiet for one interval, not
// fail-halt): a calendar API blip must not halt trading.
//
// Runs until ctx is done; call as a goroutine from main.
func RunFeedStalenessTicker(ctx context.Context, interval time.Duration, marketOpen func() (bool, error), hasRealtimePlayground func() bool) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Infof("safety: feed-staleness evaluation ticker started (every %s; suppressed while the market is closed or no realtime playground is loaded)", interval)
	for {
		select {
		case <-ctx.Done():
			log.Info("safety: feed-staleness evaluation ticker stopping")
			return
		case <-ticker.C:
			runStalenessEvaluationCycle(marketOpen, hasRealtimePlayground)
		}
	}
}

// runStalenessEvaluationCycle applies the gating for one ticker cycle and
// returns whether the guard was actually evaluated (exercised directly by
// unit tests; the ticker discards the result).
func runStalenessEvaluationCycle(marketOpen func() (bool, error), hasRealtimePlayground func() bool) bool {
	open, err := marketOpen()
	if err != nil {
		log.Warnf("safety: skipping feed-staleness evaluation cycle — market-calendar check failed: %v", err)
		return false
	}
	if !open {
		return false
	}
	if hasRealtimePlayground == nil || !hasRealtimePlayground() {
		return false
	}

	EvaluateFeedStalenessGuard()
	return true
}
