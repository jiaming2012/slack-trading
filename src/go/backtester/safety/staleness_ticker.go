package safety

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// stalenessTickerState carries the market open/closed transition tracking for
// the feed-staleness evaluation ticker. It exists to solve the market-open
// boundary problem: a server that sat through the overnight gap (or was
// started before the first bar of the session) holds a last-Tick age of many
// hours the instant the market opens — evaluating immediately at the open
// would spuriously auto-halt trading most mornings before the feed has had
// any chance to deliver its first bar.
//
// Chosen semantics: after every closed→open transition (a start while the
// market is already open counts as a transition), evaluation is suppressed
// for a grace window of openGrace — sized by the caller to the staleness
// threshold. After the grace window elapses, evaluation proceeds normally:
// a feed that is genuinely dead FROM the open still trips at openedAt+grace
// (unlike a "wait for the first heartbeat of the session" rule, which would
// never trip on a dead-from-open feed), and a feed that ticked after the open
// and then went quiet trips exactly per the threshold.
type stalenessTickerState struct {
	openGrace time.Duration
	wasOpen   bool
	openedAt  time.Time
}

// RunFeedStalenessTicker periodically evaluates the feed-staleness guard
// through the installed registry hook. Evaluation is suppressed:
//
//   - while the market is closed, or no realtime (Paper/Margin) Playground is
//     loaded — a quiet feed outside trading hours or on a Simulation-only
//     server can never engage the halt;
//   - for openGrace after every closed→open market transition (including a
//     start while the market is already open), so the overnight last-Tick gap
//     cannot trip the guard at the opening bell. Callers pass the staleness
//     threshold as openGrace.
//
// A failure of the market-calendar check itself skips that cycle with a Warn
// (fail-quiet for one interval, not fail-halt): a calendar API blip must not
// halt trading.
//
// Runs until ctx is done; call as a goroutine from main.
func RunFeedStalenessTicker(ctx context.Context, interval, openGrace time.Duration, marketOpen func() (bool, error), hasRealtimePlayground func() bool) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	state := &stalenessTickerState{openGrace: openGrace}

	log.Infof("safety: feed-staleness evaluation ticker started (every %s; suppressed while the market is closed, no realtime playground is loaded, or within %s of the market opening)", interval, openGrace)
	for {
		select {
		case <-ctx.Done():
			log.Info("safety: feed-staleness evaluation ticker stopping")
			return
		case <-ticker.C:
			state.runCycle(time.Now(), marketOpen, hasRealtimePlayground)
		}
	}
}

// runCycle applies the gating for one ticker cycle at time now and returns
// whether the guard was actually evaluated (exercised directly by unit tests;
// the ticker discards the result).
func (s *stalenessTickerState) runCycle(now time.Time, marketOpen func() (bool, error), hasRealtimePlayground func() bool) bool {
	open, err := marketOpen()
	if err != nil {
		// Fail-quiet for one interval. The open/closed transition state is
		// left untouched, so a calendar blip neither resets nor fakes an open
		// transition.
		log.Warnf("safety: skipping feed-staleness evaluation cycle — market-calendar check failed: %v", err)
		return false
	}
	if !open {
		s.wasOpen = false
		return false
	}

	// Closed→open transition (or the first open cycle after startup): start
	// the grace window during which the overnight / pre-first-bar gap is not
	// an anomaly.
	if !s.wasOpen {
		s.wasOpen = true
		s.openedAt = now
		log.Infof("safety: market opened — feed-staleness evaluation suppressed until %s (grace %s) so the overnight gap cannot trip the guard", now.Add(s.openGrace).Format(time.RFC3339), s.openGrace)
	}
	if now.Before(s.openedAt.Add(s.openGrace)) {
		return false
	}

	if hasRealtimePlayground == nil || !hasRealtimePlayground() {
		return false
	}

	EvaluateFeedStalenessGuard()
	return true
}
