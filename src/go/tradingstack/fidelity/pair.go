package fidelity

import (
	"fmt"
	"sort"
)

// Pair is a matched sim-vs-live trade pair sharing the same pairing key
// (strategy id, symbol, entry timestamp).
type Pair struct {
	Live Trade
	Sim  Trade
}

// Unmatched holds the live and simulator trades that could not be paired. These
// are reported for visibility and are deliberately excluded from every drift
// computation.
type Unmatched struct {
	Live []Trade
	Sim  []Trade
}

// pairingKey builds the stable key that identifies a candidate pair: strategy
// id, symbol, and entry timestamp (nanosecond precision, UTC-normalized so two
// equal instants in different locations key identically).
func pairingKey(t Trade) string {
	return fmt.Sprintf("%s|%s|%d", t.StrategyID, t.Symbol, t.EntryAt.UTC().UnixNano())
}

// tradeLess is a total ordering over trades used to make pairing deterministic
// regardless of input order. It orders first by pairing key, then by the
// remaining value fields to break ties among trades that share a key.
func tradeLess(a, b Trade) bool {
	ka, kb := pairingKey(a), pairingKey(b)
	if ka != kb {
		return ka < kb
	}
	if a.EntryFill != b.EntryFill {
		return a.EntryFill < b.EntryFill
	}
	if a.ExitFill != b.ExitFill {
		return a.ExitFill < b.ExitFill
	}
	if a.PnL != b.PnL {
		return a.PnL < b.PnL
	}
	if a.HoldDuration != b.HoldDuration {
		return a.HoldDuration < b.HoldDuration
	}
	return a.ExitReason < b.ExitReason
}

// PairTrades deterministically matches each live trade to at most one simulator
// trade by pairing key (strategy id, symbol, entry timestamp). The same inputs
// always yield the same set of pairs and the same unmatched sets, independent of
// input ordering: both slices are sorted by a total ordering before matching.
// Trades with no counterpart on the other side are returned as unmatched and do
// not contribute to any drift computation.
func PairTrades(live, sim []Trade) ([]Pair, Unmatched) {
	sortedLive := append([]Trade(nil), live...)
	sortedSim := append([]Trade(nil), sim...)
	sort.SliceStable(sortedLive, func(i, j int) bool { return tradeLess(sortedLive[i], sortedLive[j]) })
	sort.SliceStable(sortedSim, func(i, j int) bool { return tradeLess(sortedSim[i], sortedSim[j]) })

	// Group each side by pairing key while preserving the sorted order within a
	// key. Keys are collected as a sorted union so iteration is deterministic.
	liveByKey := map[string][]Trade{}
	simByKey := map[string][]Trade{}
	keySet := map[string]struct{}{}
	for _, t := range sortedLive {
		k := pairingKey(t)
		liveByKey[k] = append(liveByKey[k], t)
		keySet[k] = struct{}{}
	}
	for _, t := range sortedSim {
		k := pairingKey(t)
		simByKey[k] = append(simByKey[k], t)
		keySet[k] = struct{}{}
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []Pair
	var unmatched Unmatched
	for _, k := range keys {
		lgroup := liveByKey[k]
		sgroup := simByKey[k]
		n := len(lgroup)
		if len(sgroup) < n {
			n = len(sgroup)
		}
		for i := 0; i < n; i++ {
			pairs = append(pairs, Pair{Live: lgroup[i], Sim: sgroup[i]})
		}
		unmatched.Live = append(unmatched.Live, lgroup[n:]...)
		unmatched.Sim = append(unmatched.Sim, sgroup[n:]...)
	}

	return pairs, unmatched
}
