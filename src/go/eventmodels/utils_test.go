package eventmodels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	t.Run("valid duration", func(t *testing.T) {
		months, days, hours, err := ParseDuration("1m2d")
		require.NoError(t, err)
		require.Equal(t, 1, months)
		require.Equal(t, 2, days)
		require.Equal(t, 0, hours)
	})

	t.Run("valid duration, no days", func(t *testing.T) {
		months, days, hours, err := ParseDuration("1m")
		require.NoError(t, err)
		require.Equal(t, 1, months)
		require.Equal(t, 0, days)
		require.Equal(t, 0, hours)
	})

	t.Run("valid duration, no months", func(t *testing.T) {
		months, days, hours, err := ParseDuration("2d")
		require.NoError(t, err)
		require.Equal(t, 0, months)
		require.Equal(t, 2, days)
		require.Equal(t, 0, hours)
	})

	t.Run("invalid duration", func(t *testing.T) {
		_, _, _, err := ParseDuration("1")
		require.Error(t, err)
	})
}
