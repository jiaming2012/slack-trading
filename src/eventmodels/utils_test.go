package eventmodels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDuration(t *testing.T) {
	t.Run("valid duration", func(t *testing.T) {
		months, days, hours, err := ParseDuration("1m2d")
		assert.NoError(t, err)
		assert.Equal(t, 1, months)
		assert.Equal(t, 2, days)
		assert.Equal(t, 0, hours)
	})

	t.Run("valid duration, no days", func(t *testing.T) {
		months, days, hours, err := ParseDuration("1m")
		assert.NoError(t, err)
		assert.Equal(t, 1, months)
		assert.Equal(t, 0, days)
		assert.Equal(t, 0, hours)
	})

	t.Run("valid duration, no months", func(t *testing.T) {
		months, days, hours, err := ParseDuration("2d")
		assert.NoError(t, err)
		assert.Equal(t, 0, months)
		assert.Equal(t, 2, days)
		assert.Equal(t, 0, hours)
	})

	t.Run("invalid duration", func(t *testing.T) {
		_, _, _, err := ParseDuration("1")
		assert.Error(t, err)
	})
}
