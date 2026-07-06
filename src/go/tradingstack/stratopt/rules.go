package stratopt

import (
	"fmt"
	"sort"
)

// Rule names (proposal provenance) and the parameters they adjust.
const (
	RuleStopChurn   = "stop_churn"
	RuleTimeoutDrag = "timeout_drag"
	RuleFastTarget  = "fast_target"

	ParamStopPct     = "stop_pct"
	ParamMaxHoldDays = "max_hold_days"
	ParamTargetPct   = "target_pct"
)

// triggerStatName maps a rule to the name of its trigger statistic -- the
// per-fold parameter the gate's cross-fold stability check verifies.
func triggerStatName(rule string) string {
	switch rule {
	case RuleStopChurn:
		return "stop_churn_share"
	case RuleTimeoutDrag:
		return "timeout_share"
	case RuleFastTarget:
		return "fast_target_share"
	}
	return rule + "_share"
}

// Candidate is one rule-generated proposal candidate: a bounded, signed
// relative adjustment to a single execution parameter of one
// (strategy_id, regime) group, before the overfitting gate has judged it.
type Candidate struct {
	StrategyID    string
	Regime        string
	Parameter     string
	AdjustmentPct float64
	Rule          string
	Rationale     string

	// TriggerShare is the observed weighted exit-reason share that fired the
	// rule (carried into the proposal evidence).
	TriggerShare float64
}

// GenerateCandidates applies the three v1 rules to every group that clears
// the minimum-sample pre-filter (cfg.MinGroupSamples decided outcomes); a
// group below the pre-filter generates nothing. At most one candidate per
// rule per group per run; steps are fixed (no compounding). Deterministic:
// identical groups and config produce identical candidates, sorted by
// (strategy_id, regime, parameter).
func GenerateCandidates(groups []GroupStats, cfg OptimizerConfig) []Candidate {
	var candidates []Candidate

	for _, g := range groups {
		if g.DecidedCount < cfg.MinGroupSamples {
			continue
		}

		// R1 stop-churn -> widen stop: losses dominated by quick stop-outs
		// suggest the stop is inside the noise band.
		if g.QuickStopShareOfLosses >= cfg.StopChurnThreshold {
			candidates = append(candidates, Candidate{
				StrategyID:    g.StrategyID,
				Regime:        g.Regime,
				Parameter:     ParamStopPct,
				AdjustmentPct: cfg.StopStepPct,
				Rule:          RuleStopChurn,
				Rationale: fmt.Sprintf(
					"%.0f%% (weighted) of %s/%s losses exited via stop within %d day(s) -- the stop looks too tight; propose widening stop_pct by %+.0f%% (relative)",
					g.QuickStopShareOfLosses*100, g.StrategyID, g.Regime, cfg.StopChurnMaxHoldDays, cfg.StopStepPct*100),
				TriggerShare: g.QuickStopShareOfLosses,
			})
		}

		// R2 timeout-drag -> shorten hold: losses dominated by timeouts
		// suggest the catalyst is not materializing within the hold window.
		if g.TimeoutShareOfLosses >= cfg.TimeoutThreshold {
			candidates = append(candidates, Candidate{
				StrategyID:    g.StrategyID,
				Regime:        g.Regime,
				Parameter:     ParamMaxHoldDays,
				AdjustmentPct: cfg.HoldStepPct,
				Rule:          RuleTimeoutDrag,
				Rationale: fmt.Sprintf(
					"%.0f%% (weighted) of %s/%s losses exited via timeout -- losing trades drag to the hold limit; propose shortening max_hold_days by %+.0f%% (relative)",
					g.TimeoutShareOfLosses*100, g.StrategyID, g.Regime, cfg.HoldStepPct*100),
				TriggerShare: g.TimeoutShareOfLosses,
			})
		}

		// R3 fast-target -> raise target: wins dominated by fast target hits
		// suggest the target leaves gains on the table.
		if g.FastTargetShareOfWins >= cfg.FastTargetThreshold {
			candidates = append(candidates, Candidate{
				StrategyID:    g.StrategyID,
				Regime:        g.Regime,
				Parameter:     ParamTargetPct,
				AdjustmentPct: cfg.TargetStepPct,
				Rule:          RuleFastTarget,
				Rationale: fmt.Sprintf(
					"%.0f%% (weighted) of %s/%s wins exited via target within %d day(s) -- winners hit the target fast; propose raising target_pct by %+.0f%% (relative)",
					g.FastTargetShareOfWins*100, g.StrategyID, g.Regime, cfg.FastTargetMaxHoldDays, cfg.TargetStepPct*100),
				TriggerShare: g.FastTargetShareOfWins,
			})
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.StrategyID != b.StrategyID {
			return a.StrategyID < b.StrategyID
		}
		if a.Regime != b.Regime {
			return a.Regime < b.Regime
		}
		return a.Parameter < b.Parameter
	})

	return candidates
}
