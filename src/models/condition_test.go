package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExitCondition_IsSatisfied(t *testing.T) {
	name := "test"
	symbol := "symbol"
	balance := 1000.0
	newUpPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price:             1.0,
				StopLoss:          0.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          1.5,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}
	}

	strategy, _err := NewStrategy(name, symbol, Up, balance, newUpPriceLevels())
	assert.NoError(t, _err)

	t.Run("returns false when no signals are set", func(t *testing.T) {
		c, err := NewExitCondition(0, nil, nil, 1)
		assert.NoError(t, err)
		assert.False(t, c.IsSatisfied(strategy))
	})

	t.Run("1 signal", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}
		c, err := NewExitCondition(0, signals, nil, 1)
		assert.NoError(t, err)

		s1.IsSatisfied = true
		assert.True(t, c.IsSatisfied(strategy))
	})

	t.Run("2 signal", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		s2 := NewSignalV2("signal2")
		signals := []*SignalV2{s2, s1}
		c, err := NewExitCondition(0, signals, nil, 1)
		assert.NoError(t, err)

		s1.IsSatisfied = true
		assert.False(t, c.IsSatisfied(strategy))

		s2.IsSatisfied = true
		assert.True(t, c.IsSatisfied(strategy))
	})

	t.Run("not satisfied when one constraint is false", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}
		constraintReturnValue := false
		c1 := NewSignalConstraint("constraint1", func(s *Strategy) bool {
			return constraintReturnValue
		})
		constraints := []*SignalConstraint{c1}

		c, err := NewExitCondition(0, signals, constraints, 1)
		assert.NoError(t, err)

		s1.IsSatisfied = true
		assert.False(t, c.IsSatisfied(strategy))

		constraintReturnValue = true
		assert.True(t, c.IsSatisfied(strategy))
	})

	t.Run("satisfied when both constraints are true", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}
		constraintReturnValue1 := false
		constraintReturnValue2 := false
		c1 := NewSignalConstraint("constraint1", func(s *Strategy) bool {
			return constraintReturnValue1
		})
		c2 := NewSignalConstraint("constraint2", func(s *Strategy) bool {
			return constraintReturnValue2
		})
		constraints := []*SignalConstraint{c1, c2}

		c, err := NewExitCondition(0, signals, constraints, 1)
		assert.NoError(t, err)

		s1.IsSatisfied = true
		assert.False(t, c.IsSatisfied(strategy))

		constraintReturnValue1 = true
		assert.False(t, c.IsSatisfied(strategy))

		constraintReturnValue2 = true
		assert.True(t, c.IsSatisfied(strategy))
	})
}
