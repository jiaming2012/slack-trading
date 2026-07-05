package costmodel

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadCostParams_DefaultsWhenFileAbsent pins the spec scenario: when no
// file exists at the resolved path, the loader returns the built-in
// defaults and a nil error.
func TestLoadCostParams_DefaultsWhenFileAbsent(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	got, err := LoadCostParams(missing)
	if err != nil {
		t.Fatalf("LoadCostParams returned error: %v", err)
	}

	want := DefaultCostParams()
	if got != want {
		t.Fatalf("LoadCostParams(missing file) = %+v, want defaults %+v", got, want)
	}
}

// TestLoadCostParams_YAMLOverridesFieldByField pins the spec scenario: a
// YAML file that sets avg_spread_bps and commission_per_share but omits
// borrow_rate_bps_annual yields a CostParams with the two overrides applied
// and the built-in default retained for the omitted field.
func TestLoadCostParams_YAMLOverridesFieldByField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cost-config.yaml")

	yamlContent := "avg_spread_bps: 8\ncommission_per_share: 0.02\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	got, err := LoadCostParams(path)
	if err != nil {
		t.Fatalf("LoadCostParams returned error: %v", err)
	}

	defaults := DefaultCostParams()

	if got.AvgSpreadBps != 8 {
		t.Fatalf("AvgSpreadBps = %v, want 8", got.AvgSpreadBps)
	}
	if got.CommissionPerShare != 0.02 {
		t.Fatalf("CommissionPerShare = %v, want 0.02", got.CommissionPerShare)
	}
	if got.BorrowRateBpsAnnual != defaults.BorrowRateBpsAnnual {
		t.Fatalf("BorrowRateBpsAnnual = %v, want default %v (unset field must retain default)", got.BorrowRateBpsAnnual, defaults.BorrowRateBpsAnnual)
	}
	if got.CommissionPerTrade != defaults.CommissionPerTrade {
		t.Fatalf("CommissionPerTrade = %v, want default %v (unset field must retain default)", got.CommissionPerTrade, defaults.CommissionPerTrade)
	}
}

// TestLoadCostParams_ImpactParamsLoadAlongsideCostParams pins the
// strategy-capacity-estimate spec scenario: impact_coefficient and
// max_impact_fraction_of_edge load from the same YAML file and loader as
// the commission/spread/borrow fields.
func TestLoadCostParams_ImpactParamsLoadAlongsideCostParams(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cost-config.yaml")

	yamlContent := "impact_coefficient: 0.15\nmax_impact_fraction_of_edge: 0.3\ncommission_per_trade: 2.50\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	got, err := LoadCostParams(path)
	if err != nil {
		t.Fatalf("LoadCostParams returned error: %v", err)
	}

	if got.ImpactCoefficient != 0.15 {
		t.Fatalf("ImpactCoefficient = %v, want 0.15", got.ImpactCoefficient)
	}
	if got.MaxImpactFractionOfEdge != 0.3 {
		t.Fatalf("MaxImpactFractionOfEdge = %v, want 0.3", got.MaxImpactFractionOfEdge)
	}
	if got.CommissionPerTrade != 2.50 {
		t.Fatalf("CommissionPerTrade = %v, want 2.50", got.CommissionPerTrade)
	}
}
