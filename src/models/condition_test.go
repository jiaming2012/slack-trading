package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	name := "ExitCondition"
	levels := newUpPriceLevels()
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)

	newResetConditions := func() []*SignalV2 {
		return []*SignalV2{NewSignalV2("reset", ts)}
	}

	t.Run("returns false when no signals are set", func(t *testing.T) {
		c, err := NewExitCondition(name, 0, nil, nil, nil, 1, nil)
		require.NoError(t, err)

		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		isSatisfied, err := c.IsSatisfied(levels[0], params)
		require.NoError(t, err)
		require.False(t, isSatisfied)
	})

	t.Run("1 signal", func(t *testing.T) {
		s1 := NewSignalV2("signal1", ts)
		r1 := NewResetSignal("reset1", s1, ts)

		signals := []*ExitSignal{NewExitSignal(s1, r1)}
		c, err := NewExitCondition(name, 0, signals, newResetConditions(), nil, 1, nil)
		require.NoError(t, err)

		s1.isSatisfied = true
		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		isSatisfied, err := c.IsSatisfied(levels[0], params)
		require.NoError(t, err)

		require.True(t, isSatisfied)
	})

	t.Run("2 signal", func(t *testing.T) {
		s1 := NewSignalV2("signal1", ts)
		r1 := NewResetSignal("reset1", s1, ts)
		s2 := NewSignalV2("signal2", ts)
		r2 := NewResetSignal("reset2", s1, ts)

		signals := []*ExitSignal{NewExitSignal(s2, r2), NewExitSignal(s1, r1)}

		c, err := NewExitCondition(name, 0, signals, newResetConditions(), nil, 1, nil)
		require.NoError(t, err)

		isSatisfied, err := c.IsSatisfied(levels[0], nil)
		require.NoError(t, err)

		s1.isSatisfied = true
		require.False(t, isSatisfied)

		s2.isSatisfied = true

		isSatisfied, err = c.IsSatisfied(levels[0], nil)
		require.NoError(t, err)

		require.True(t, isSatisfied)
	})

	t.Run("not satisfied when one constraint is false", func(t *testing.T) {
		s1 := NewSignalV2("signal1", ts)
		reset1 := NewResetSignal("reset1", s1, ts)
		resetSignals := []*SignalV2{NewSignalV2("reset", ts)}

		signals := []*ExitSignal{NewExitSignal(s1, reset1)}

		constraintReturnValue := false
		c1 := NewExitSignalConstraint("constraint1", func(p *PriceLevel, c *ExitCondition, params map[string]interface{}) (bool, error) {
			return constraintReturnValue, nil
		})
		constraints := []*ExitSignalConstraint{c1}

		c, err := NewExitCondition(name, 0, signals, resetSignals, constraints, 1, nil)
		require.NoError(t, err)

		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		isSatisfied, err := c.IsSatisfied(levels[0], params)
		require.NoError(t, err)

		s1.isSatisfied = true
		require.False(t, isSatisfied)

		constraintReturnValue = true

		isSatisfied, err = c.IsSatisfied(levels[0], params)
		require.NoError(t, err)

		require.True(t, isSatisfied)
	})

	t.Run("satisfied when both constraints are true", func(t *testing.T) {
		s1 := NewSignalV2("signal1", ts)
		r1 := NewResetSignal("reset1", s1, ts)

		signals := []*ExitSignal{NewExitSignal(s1, r1)}

		constraintReturnValue1 := false
		constraintReturnValue2 := false
		c1 := NewExitSignalConstraint("constraint1", func(p *PriceLevel, c *ExitCondition, params map[string]interface{}) (bool, error) {
			return constraintReturnValue1, nil
		})
		c2 := NewExitSignalConstraint("constraint2", func(p *PriceLevel, c *ExitCondition, params map[string]interface{}) (bool, error) {
			return constraintReturnValue2, nil
		})
		constraints := []*ExitSignalConstraint{c1, c2}

		c, err := NewExitCondition(name, 0, signals, newResetConditions(), constraints, 1, nil)
		require.NoError(t, err)

		params := map[string]interface{}{"tick": Tick{Bid: 1.0, Ask: 1.0}}
		s1.isSatisfied = true

		isSatisfied, err := c.IsSatisfied(levels[0], params)
		require.NoError(t, err)
		require.False(t, isSatisfied)

		constraintReturnValue1 = true

		isSatisfied, err = c.IsSatisfied(levels[0], params)
		require.NoError(t, err)
		require.False(t, isSatisfied)

		constraintReturnValue2 = true

		isSatisfied, err = c.IsSatisfied(levels[0], params)
		require.NoError(t, err)
		require.True(t, isSatisfied)
	})

	t.Run("reset signals", func(t *testing.T) {
		s1 := NewSignalV2("signal1", ts)
		r1 := NewResetSignal("reset1", s1, ts)

		signals := []*ExitSignal{NewExitSignal(s1, r1)}
		resetConditions := newResetConditions()

		c, err := NewExitCondition(name, 0, signals, resetConditions, nil, 1, nil)
		require.NoError(t, err)

		isSatisfied, err := c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.False(t, isSatisfied)

		s1.isSatisfied = true

		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.True(t, isSatisfied)

		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.False(t, isSatisfied) // state should change automatically

		s1.isSatisfied = false

		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.False(t, isSatisfied)

		s1.isSatisfied = true

		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.False(t, isSatisfied) // reset condition still not satisfied

		resetConditions[0].isSatisfied = true // reset condition is satisfied

		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.True(t, isSatisfied)
	})

	t.Run("max number of triggers", func(t *testing.T) {
		s1 := NewSignalV2("signal1", ts)
		r1 := NewResetSignal("reset1", s1, ts)

		signals := []*ExitSignal{NewExitSignal(s1, r1)}
		resetConditions := newResetConditions()

		maxTriggerCount := 2
		c, err := NewExitCondition(name, 0, signals, resetConditions, nil, 1, &maxTriggerCount)
		require.NoError(t, err)

		require.Equal(t, 0, c.TriggerCount)

		s1.isSatisfied = true

		isSatisfied, err := c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.True(t, isSatisfied)

		require.Equal(t, 1, c.TriggerCount)

		// reset: count = 1
		s1.isSatisfied = false

		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.False(t, isSatisfied)
		require.Equal(t, 1, c.TriggerCount)

		s1.isSatisfied = true
		resetConditions[0].isSatisfied = true
		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.True(t, isSatisfied)

		// reset: count = 2
		s1.isSatisfied = false
		resetConditions[0].isSatisfied = true
		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.False(t, isSatisfied)
		require.Equal(t, 2, c.TriggerCount)

		s1.isSatisfied = true
		resetConditions[0].isSatisfied = true
		isSatisfied, err = c.IsSatisfied(nil, nil)
		require.NoError(t, err)
		require.False(t, isSatisfied)
	})
}
