package models

import "fmt"

type SignalConstraints []*ExitSignalConstraint

func (constraints SignalConstraints) Validate() error {
	names := make(map[string]struct{})
	for _, c := range constraints {
		if _, exists := names[c.Name]; exists {
			return fmt.Errorf("SignalConstraints.Validate: duplicate name not allowed, found %v twice", c.Name)
		}
		names[c.Name] = struct{}{}
	}

	return nil
}

type ExitSignalConstraint struct {
	Name  string                                                                  `json:"name"`
	Check func(*PriceLevel, *ExitCondition, map[string]interface{}) (bool, error) `json:"-"`
}

func NewExitSignalConstraint(name string, check func(level *PriceLevel, condition *ExitCondition, params map[string]interface{}) (bool, error)) *ExitSignalConstraint {
	return &ExitSignalConstraint{Name: name, Check: check}
}

func PriceLevelProfitLossAboveZeroConstraint(priceLevel *PriceLevel, _ *ExitCondition, params map[string]interface{}) (bool, error) {
	tick := params["tick"].(Tick)
	stats, err := priceLevel.Trades.GetTradeStats(tick)
	if err != nil {
		return false, fmt.Errorf("PriceLevelProfitLossAboveZeroConstraint: failed to get trade stats: %w", err)
	}

	pl := stats.RealizedPL + stats.FloatingPL

	return pl > 0, nil
}
