package eventmodels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRiskProfileConstrain(t *testing.T) {
	t.Run("AddItem", func(t *testing.T) {
		profile := RiskProfileConstraint{}
		profile.AddItem(1.0, 2.0)
		profile.AddItem(2.0, 3.0)
		profile.AddItem(3.0, 4.0)

		require.Equal(t, 3, len(profile.items))
	})

	t.Run("GetMaxRisk", func(t *testing.T) {
		profile := RiskProfileConstraint{}
		profile.AddItem(1.0, 2.0)
		profile.AddItem(2.0, 3.0)
		profile.AddItem(3.0, 4.0)

		maxRisk, err := profile.GetMaxRisk(2.5)
		require.Nil(t, err)
		require.Equal(t, 2.0, maxRisk)

		maxRisk, err = profile.GetMaxRisk(3.5)
		require.Nil(t, err)
		require.Equal(t, 3.0, maxRisk)

		maxRisk, err = profile.GetMaxRisk(4.5)
		require.Nil(t, err)
		require.Equal(t, 3.0, maxRisk)
	})

	t.Run("GetMaxRisk (empty profile)", func(t *testing.T) {
		profile := RiskProfileConstraint{}

		maxRisk, err := profile.GetMaxRisk(2.5)
		require.NotNil(t, err)
		require.Equal(t, 0.0, maxRisk)
	})

	t.Run("GetMaxRisk (below min risk)", func(t *testing.T) {
		profile := RiskProfileConstraint{}
		profile.AddItem(1.0, 2.0)
		profile.AddItem(2.0, 3.0)
		profile.AddItem(3.0, 4.0)

		maxRisk, err := profile.GetMaxRisk(0.5)
		require.NotNil(t, err)
		require.Equal(t, 0.0, maxRisk)
	})

	t.Run("GetMaxRisk (above max risk)", func(t *testing.T) {
		profile := RiskProfileConstraint{}
		profile.AddItem(1.0, 2.0)
		profile.AddItem(2.0, 3.0)
		profile.AddItem(3.0, 4.0)

		maxRisk, err := profile.GetMaxRisk(5.0)
		require.Nil(t, err)
		require.Equal(t, 3.0, maxRisk)
	})
}
