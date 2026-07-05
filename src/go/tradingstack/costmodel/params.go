// Package costmodel computes cost-adjusted (net) expected value for closed
// trades and estimates strategy capacity via a square-root market-impact
// model. It is a pure-calculation library: no RPC surface, no background
// job. A future ev-tracker aggregation pipeline is expected to call
// ComputeEV and EstimateCapacity when rolling up strategy/regime EV windows.
package costmodel

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CostParams carries the cost and market-impact assumptions used to turn a
// closed trade's gross P&L into a cost-adjusted (net) P&L, and to estimate
// how much size a strategy can carry before its own market impact erodes
// its edge.
type CostParams struct {
	// CommissionPerTrade is a flat commission charged per closed trade.
	CommissionPerTrade float64 `yaml:"commission_per_trade"`
	// CommissionPerShare is an additional commission charged per share traded.
	CommissionPerShare float64 `yaml:"commission_per_share"`
	// AvgSpreadBps is the assumed average bid/ask spread, in basis points of
	// entry price, charged once per trade as an estimated spread cost.
	AvgSpreadBps float64 `yaml:"avg_spread_bps"`
	// BorrowRateBpsAnnual is the assumed annualized short-borrow rate, in
	// basis points, applied only to short trades and accrued over hold days.
	BorrowRateBpsAnnual float64 `yaml:"borrow_rate_bps_annual"`
	// ImpactCoefficient is the dimensionless square-root market-impact
	// constant used by EstimateCapacity.
	ImpactCoefficient float64 `yaml:"impact_coefficient"`
	// MaxImpactFractionOfEdge is the fraction X, in [0, 1], of expected
	// per-share edge that estimated market impact is allowed to consume
	// before capacity is considered exhausted.
	MaxImpactFractionOfEdge float64 `yaml:"max_impact_fraction_of_edge"`
}

// DefaultCostParams returns the built-in default cost and market-impact
// assumptions, used whenever no override YAML file is present.
func DefaultCostParams() CostParams {
	return CostParams{
		CommissionPerTrade:      1.00,
		CommissionPerShare:      0.005,
		AvgSpreadBps:            5,
		BorrowRateBpsAnnual:     300,
		ImpactCoefficient:       0.1,
		MaxImpactFractionOfEdge: 0.2,
	}
}

// LoadCostParams resolves a config path — the explicit path argument if
// non-empty, else the NET_EV_COST_CONFIG_PATH environment variable, else
// ${TRADING_PROJECT_DIR}/src/go/net-ev-cost-config.yaml — and loads YAML
// overrides on top of DefaultCostParams. If no file exists at the resolved
// path, it returns the built-in defaults and a nil error. Fields omitted
// from the YAML file retain their default values.
func LoadCostParams(path string) (CostParams, error) {
	resolved := resolveCostConfigPath(path)
	params := DefaultCostParams()

	if resolved == "" {
		return params, nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return params, nil
		}
		return CostParams{}, fmt.Errorf("costmodel: LoadCostParams: failed to read %q: %w", resolved, err)
	}

	if err := yaml.Unmarshal(data, &params); err != nil {
		return CostParams{}, fmt.Errorf("costmodel: LoadCostParams: failed to parse %q: %w", resolved, err)
	}

	return params, nil
}

// resolveCostConfigPath implements the path-resolution order documented on
// LoadCostParams. It returns "" only when no path could be resolved at all
// (no explicit path, no env var, and no TRADING_PROJECT_DIR), in which case
// the caller falls back to built-in defaults.
func resolveCostConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}

	if envPath := os.Getenv("NET_EV_COST_CONFIG_PATH"); envPath != "" {
		return envPath
	}

	if projectDir := os.Getenv("TRADING_PROJECT_DIR"); projectDir != "" {
		return projectDir + "/src/go/net-ev-cost-config.yaml"
	}

	return ""
}
