package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func newUpPriceLevels() []*PriceLevel {
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

func TestExitCondition_IsSatisfied(t *testing.T) {
	levels := newUpPriceLevels()
	resetConditions := []*SignalV2{NewSignalV2("reset")}
	//strategy, _err := NewStrategy(name, symbol, Up, balance, levels, nil)
	//assert.NoError(t, _err)

	t.Run("returns false when no signals are set", func(t *testing.T) {
		c, err := NewExitCondition(0, nil, nil, nil, 1, nil)
		assert.NoError(t, err)

		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		isSatisfied, err := c.IsSatisfied(levels[0], params)
		assert.False(t, isSatisfied)
	})

	t.Run("1 signal", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}
		c, err := NewExitCondition(0, signals, resetConditions, nil, 1, nil)
		assert.NoError(t, err)

		s1.IsSatisfied = true
		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		isSatisfied, err := c.IsSatisfied(levels[0], params)
		assert.NoError(t, err)

		assert.True(t, isSatisfied)
	})

	t.Run("2 signal", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		s2 := NewSignalV2("signal2")
		signals := []*SignalV2{s2, s1}

		c, err := NewExitCondition(0, signals, resetConditions, nil, 1, nil)
		assert.NoError(t, err)

		isSatisfied, err := c.IsSatisfied(levels[0], nil)
		assert.NoError(t, err)

		s1.IsSatisfied = true
		assert.False(t, isSatisfied)

		s2.IsSatisfied = true

		isSatisfied, err = c.IsSatisfied(levels[0], nil)
		assert.NoError(t, err)

		assert.True(t, isSatisfied)
	})

	t.Run("not satisfied when one constraint is false", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}
		reset1 := NewSignalV2("reset1")
		resetSignals := []*SignalV2{reset1}
		constraintReturnValue := false
		c1 := NewExitSignalConstraint("constraint1", func(p *PriceLevel, c *ExitCondition, params map[string]interface{}) (bool, error) {
			return constraintReturnValue, nil
		})
		constraints := []*ExitSignalConstraint{c1}

		c, err := NewExitCondition(0, signals, resetSignals, constraints, 1, nil)
		assert.NoError(t, err)

		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		isSatisfied, err := c.IsSatisfied(levels[0], params)
		assert.NoError(t, err)

		s1.IsSatisfied = true
		assert.False(t, isSatisfied)

		constraintReturnValue = true

		isSatisfied, err = c.IsSatisfied(levels[0], params)
		assert.NoError(t, err)

		assert.True(t, isSatisfied)
	})

	t.Run("satisfied when both constraints are true", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}
		constraintReturnValue1 := false
		constraintReturnValue2 := false
		c1 := NewExitSignalConstraint("constraint1", func(p *PriceLevel, c *ExitCondition, params map[string]interface{}) (bool, error) {
			return constraintReturnValue1, nil
		})
		c2 := NewExitSignalConstraint("constraint2", func(p *PriceLevel, c *ExitCondition, params map[string]interface{}) (bool, error) {
			return constraintReturnValue2, nil
		})
		constraints := []*ExitSignalConstraint{c1, c2}

		c, err := NewExitCondition(0, signals, resetConditions, constraints, 1, nil)
		assert.NoError(t, err)

		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		s1.IsSatisfied = true

		isSatisfied, err := c.IsSatisfied(levels[0], params)
		assert.NoError(t, err)
		assert.False(t, isSatisfied)

		constraintReturnValue1 = true

		isSatisfied, err = c.IsSatisfied(levels[0], params)
		assert.NoError(t, err)
		assert.False(t, isSatisfied)

		constraintReturnValue2 = true

		isSatisfied, err = c.IsSatisfied(levels[0], params)
		assert.NoError(t, err)
		assert.True(t, isSatisfied)
	})

	t.Run("reset signals", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}

		c, err := NewExitCondition(0, signals, resetConditions, nil, 1, nil)
		assert.NoError(t, err)

		isSatisfied, err := c.IsSatisfied(nil, nil)
		assert.NoError(t, err)

		assert.False(t, isSatisfied)

		s1.IsSatisfied = true

		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.True(t, isSatisfied)

		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.False(t, isSatisfied) // state should change automatically

		s1.IsSatisfied = false

		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.False(t, isSatisfied)

		s1.IsSatisfied = true

		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.True(t, isSatisfied)
	})

	t.Run("max number of triggers", func(t *testing.T) {
		s1 := NewSignalV2("signal1")
		signals := []*SignalV2{s1}

		maxTriggerCount := 2
		c, err := NewExitCondition(0, signals, resetConditions, nil, 1, &maxTriggerCount)
		assert.NoError(t, err)

		assert.Equal(t, 0, c.TriggerCount)

		s1.IsSatisfied = true

		isSatisfied, err := c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.True(t, isSatisfied)

		assert.Equal(t, 1, c.TriggerCount)

		// reset: count = 1
		s1.IsSatisfied = false

		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.False(t, isSatisfied)
		assert.Equal(t, 1, c.TriggerCount)

		s1.IsSatisfied = true
		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.True(t, isSatisfied)

		// reset: count = 2
		s1.IsSatisfied = false
		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.False(t, isSatisfied)
		assert.Equal(t, 2, c.TriggerCount)

		s1.IsSatisfied = true
		isSatisfied, err = c.IsSatisfied(nil, nil)
		assert.NoError(t, err)
		assert.False(t, isSatisfied)
	})
}
