package costmodel

import "testing"

func capacityFixtureParams() CostParams {
	return CostParams{
		ImpactCoefficient:       0.1,
		MaxImpactFractionOfEdge: 0.2,
	}
}

// TestEstimateCapacity_HandComputedFixture pins the spec's fixture:
// edge=2.00, price=50, ADV=1,000,000, impact_coefficient=0.1,
// max_impact_fraction_of_edge=0.2 -> 6400 shares.
func TestEstimateCapacity_HandComputedFixture(t *testing.T) {
	got := EstimateCapacity(2.00, 50, 1_000_000, capacityFixtureParams())
	want := 6400.0
	if got != want {
		t.Fatalf("EstimateCapacity = %v, want %v", got, want)
	}
}

// TestEstimateCapacity_ZeroEdgeYieldsZero is a boundary-condition test at
// the edge_per_share == 0 threshold: the spec requires 0, not an error or
// division-by-zero panic, exactly at the boundary (not just for negative
// values).
func TestEstimateCapacity_ZeroEdgeYieldsZero(t *testing.T) {
	got := EstimateCapacity(0, 50, 1_000_000, capacityFixtureParams())
	if got != 0 {
		t.Fatalf("EstimateCapacity(edge=0) = %v, want 0", got)
	}
}

// TestEstimateCapacity_NegativeEdgeYieldsZero asserts negative edge also
// yields 0, not a negative capacity or a NaN from sqrt of a negative number.
func TestEstimateCapacity_NegativeEdgeYieldsZero(t *testing.T) {
	got := EstimateCapacity(-2.00, 50, 1_000_000, capacityFixtureParams())
	if got != 0 {
		t.Fatalf("EstimateCapacity(edge=-2.00) = %v, want 0", got)
	}
}

// TestEstimateCapacity_ZeroPriceYieldsZero is a boundary-condition test at
// the price == 0 threshold.
func TestEstimateCapacity_ZeroPriceYieldsZero(t *testing.T) {
	got := EstimateCapacity(2.00, 0, 1_000_000, capacityFixtureParams())
	if got != 0 {
		t.Fatalf("EstimateCapacity(price=0) = %v, want 0", got)
	}
}

// TestEstimateCapacity_ZeroAvgDailyVolumeYieldsZero is a boundary-condition
// test at the avg_daily_volume == 0 threshold.
func TestEstimateCapacity_ZeroAvgDailyVolumeYieldsZero(t *testing.T) {
	got := EstimateCapacity(2.00, 50, 0, capacityFixtureParams())
	if got != 0 {
		t.Fatalf("EstimateCapacity(avg_daily_volume=0) = %v, want 0", got)
	}
}

// TestEstimateCapacity_DoublingADVDoublesCapacity asserts the proportionality
// scenario: doubling avg_daily_volume exactly doubles the returned max_shares,
// holding every other input fixed.
func TestEstimateCapacity_DoublingADVDoublesCapacity(t *testing.T) {
	params := capacityFixtureParams()

	first := EstimateCapacity(2.00, 50, 1_000_000, params)
	second := EstimateCapacity(2.00, 50, 2_000_000, params)

	if second != first*2 {
		t.Fatalf("doubling ADV: first=%v second=%v, want second == first*2", first, second)
	}
}
