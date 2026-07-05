// Command fidelity-check runs the simulator-vs-live fidelity checker for a
// period and prints the per-strategy drift results and gate decisions.
//
// It exits non-zero only on an execution error. A tolerance breach is a normal,
// reportable outcome and exits zero. When there are no live trades for the
// period it reports a clean "no live trades available" status and exits zero.
//
// The --synthetic flag runs the checker against a built-in synthetic sim-vs-live
// dataset whose drift is known by construction, so the command is runnable with
// no live-trade input (real live-trade ingestion accrues in a later change).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/fidelity"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "fidelity-check: error:", err)
		os.Exit(1)
	}
}

// run parses flags, executes the checker, and writes the report to out. It
// returns a non-nil error only on an execution failure; a tolerance breach or an
// empty live set is reported to out and returns nil.
func run(args []string, out *os.File) error {
	fs := flag.NewFlagSet("fidelity-check", flag.ContinueOnError)
	periodStart := fs.String("period-start", "", "period start (YYYY-MM-DD or RFC3339)")
	periodEnd := fs.String("period-end", "", "period end (YYYY-MM-DD or RFC3339)")
	synthetic := fs.Bool("synthetic", false, "run against the built-in synthetic sim-vs-live dataset")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var set fidelity.TradeSet
	var period fidelity.Period

	if *synthetic {
		set, period = fidelity.SyntheticTradeSet()
	}

	// Explicit period flags override the (synthetic or zero) default.
	if *periodStart != "" {
		t, err := parseTime(*periodStart)
		if err != nil {
			return fmt.Errorf("parse --period-start: %w", err)
		}
		period.Start = t
	}
	if *periodEnd != "" {
		t, err := parseTime(*periodEnd)
		if err != nil {
			return fmt.Errorf("parse --period-end: %w", err)
		}
		period.End = t
	}

	results, decisions, err := fidelity.RunFidelityCheck(context.Background(), set, period, fidelity.DefaultConfig(), nil)
	if err != nil {
		if errors.Is(err, fidelity.ErrNoLiveTrades) {
			fmt.Fprintln(out, "no live trades available for the period; nothing to compare")
			return nil
		}
		return err
	}

	fmt.Fprintf(out, "Fidelity check for %s .. %s\n", period.Start.Format(time.RFC3339), period.End.Format(time.RFC3339))
	sort.Slice(results, func(i, j int) bool { return results[i].StrategyID < results[j].StrategyID })
	for _, r := range results {
		fmt.Fprintf(out, "  strategy=%s drift_score=%.4f drift_pnl=%.4f drift_fill=%.4f within_tolerance=%t decision=%s\n",
			r.StrategyID, r.DriftScore, r.DriftPnL, r.DriftFill, r.WithinTolerance, decisions[r.StrategyID])
	}
	return nil
}

// parseTime accepts either a bare date (YYYY-MM-DD, interpreted as UTC midnight)
// or a full RFC3339 timestamp.
func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
