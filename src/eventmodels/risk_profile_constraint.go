package eventmodels

import (
	"fmt"
	"sort"
)

type RiskProfileConstraintItem struct {
	MaxRisk                    float64
	RiskAdjustedExpectedProfit float64
}

type RiskProfileConstraint struct {
	items []RiskProfileConstraintItem
}

func (profile *RiskProfileConstraint) AddItem(maxRisk float64, riskAdjustedExpectedProfit float64) {
	profile.items = append(profile.items, RiskProfileConstraintItem{
		MaxRisk:                    maxRisk,
		RiskAdjustedExpectedProfit: riskAdjustedExpectedProfit,
	})

	sort.Slice(profile.items, func(i, j int) bool {
		return profile.items[i].RiskAdjustedExpectedProfit < profile.items[j].RiskAdjustedExpectedProfit
	})
}

func (profile *RiskProfileConstraint) GetMaxRisk(riskAdjustedExpectedProfit float64) (float64, error) {
	if len(profile.items) == 0 {
		return 0, fmt.Errorf("GetMaxRisk: no items in profile")
	}

	if riskAdjustedExpectedProfit < profile.items[0].RiskAdjustedExpectedProfit {
		return 0, fmt.Errorf("GetMaxRisk: riskAdjustedExpectedProfit (%v) is below minimum (%v)", riskAdjustedExpectedProfit, profile.items[0].RiskAdjustedExpectedProfit)
	}

	for _, item := range profile.items {
		if riskAdjustedExpectedProfit <= item.RiskAdjustedExpectedProfit {
			return item.MaxRisk, nil
		}
	}

	return profile.items[len(profile.items)-1].MaxRisk, nil
}

func NewRiskProfileConstraint() *RiskProfileConstraint {
	return &RiskProfileConstraint{}
}
