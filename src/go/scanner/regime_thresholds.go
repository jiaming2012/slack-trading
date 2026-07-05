package scanner

// RegimeThresholds holds the built-in default per-regime bounds used by
// RegimeFilter. These are static defaults only -- the Scanner Optimizer's
// dynamic, hot-swappable scanner_configs payload (regime-conditional feature
// weights/thresholds that can be updated at runtime) is explicitly out of
// scope for this change; see design.md.
type RegimeThresholds struct {
	// ATRPctCeiling is the maximum allowed atr_pct (as a raw ratio, e.g. 0.10
	// means 10%) for the regime. A ticker whose atr_pct exceeds this ceiling
	// is rejected by the regime filter.
	ATRPctCeiling float64

	// PriceVs50MABound is the symmetric bound (in percent, e.g. 40.0 means
	// +/-40%) on price_vs_50ma for the regime. A ticker whose price_vs_50ma
	// magnitude exceeds this bound is rejected by the regime filter.
	PriceVs50MABound float64
}

// regimeThresholdsByTag is the built-in default table. Only recognized tags
// have an entry; RegimeFilter treats any other tag (including empty) as a
// no-op, per the spec's "missing regime is a no-op" requirement.
var regimeThresholdsByTag = map[string]RegimeThresholds{
	"trending":       {ATRPctCeiling: 0.10, PriceVs50MABound: 40.0},
	"mean_reverting": {ATRPctCeiling: 0.06, PriceVs50MABound: 12.0},
	"high_vol":       {ATRPctCeiling: 0.04, PriceVs50MABound: 25.0},
}
