package costmodel

import "errors"

// ErrStrategyEvWeightsTableMissing is returned by MigrateNetEvCostModel when
// the strategy_ev_weights table does not yet exist, i.e. the
// trading-stack-schema migration (tradingstack.MigrateTradingStack) has not
// been run against this database.
var ErrStrategyEvWeightsTableMissing = errors.New("costmodel: strategy_ev_weights table does not exist; run tradingstack.MigrateTradingStack first")
