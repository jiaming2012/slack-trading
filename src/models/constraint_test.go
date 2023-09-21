package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSignalConstraints_Validate(t *testing.T) {
	t.Run("duplicate names not allowed", func(t *testing.T) {
		c1 := NewExitSignalConstraint("c1", nil)
		c2 := NewExitSignalConstraint("c2", nil)
		constraints := SignalConstraints{c1, c2}
		assert.NoError(t, constraints.Validate())

		c3 := NewExitSignalConstraint("c1", nil)
		constraints = SignalConstraints{c1, c2, c3}
		assert.Error(t, constraints.Validate())
	})
}
