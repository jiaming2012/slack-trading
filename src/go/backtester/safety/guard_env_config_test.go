package safety

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadGuardEnvConfig_Defaults(t *testing.T) {
	// t.Setenv guarantees restoration; explicitly blank every var so an
	// ambient shell value cannot leak into the assertions.
	for _, v := range []string{
		EnvGuardRejectionWindow, EnvGuardRejectionThreshold, EnvGuardRejectionMinSamples,
		EnvGuardFillDeviationPct,
		EnvGuardTradesPerHourMean, EnvGuardTradesPerHourStdDev, EnvGuardTradesPerHourMinSamp,
		EnvGuardFeedStalenessThreshold, EnvGuardStalenessEvalInterval,
	} {
		t.Setenv(v, "")
	}

	cfg, err := LoadGuardEnvConfig()
	require.NoError(t, err)

	require.True(t, cfg.RejectionEnabled)
	require.Equal(t, DefaultRejectionWindow, cfg.Guards.RejectionWindow)
	require.Equal(t, DefaultRejectionThreshold, cfg.Guards.RejectionThreshold)
	require.Equal(t, DefaultRejectionMinSamples, cfg.Guards.RejectionMinSamples)

	require.True(t, cfg.FillDeviationEnabled)
	require.Equal(t, DefaultFillDeviationPct, cfg.Guards.FillDeviationPct)

	require.True(t, cfg.TradesPerHourEnabled)
	require.Equal(t, DefaultTradesPerHourMean, cfg.Guards.TradesPerHourMean)
	require.Equal(t, DefaultTradesPerHourStdDev, cfg.Guards.TradesPerHourStdDev)
	require.Equal(t, DefaultTradesPerHourMinSamples, cfg.Guards.TradesPerHourMinSamples)

	require.True(t, cfg.FeedStalenessEnabled)
	require.Equal(t, DefaultFeedStalenessThreshold, cfg.Guards.FeedStalenessThreshold)
	require.Equal(t, DefaultStalenessEvalInterval, cfg.StalenessEvalInterval)
}

func TestLoadGuardEnvConfig_Overrides(t *testing.T) {
	t.Setenv(EnvGuardRejectionWindow, "5m")
	t.Setenv(EnvGuardRejectionThreshold, "0.25")
	t.Setenv(EnvGuardRejectionMinSamples, "10")
	t.Setenv(EnvGuardFillDeviationPct, "0.02")
	t.Setenv(EnvGuardTradesPerHourMean, "12.5")
	t.Setenv(EnvGuardTradesPerHourStdDev, "3.5")
	t.Setenv(EnvGuardTradesPerHourMinSamp, "8")
	t.Setenv(EnvGuardFeedStalenessThreshold, "90s")
	t.Setenv(EnvGuardStalenessEvalInterval, "10s")

	cfg, err := LoadGuardEnvConfig()
	require.NoError(t, err)

	require.Equal(t, 5*time.Minute, cfg.Guards.RejectionWindow)
	require.Equal(t, 0.25, cfg.Guards.RejectionThreshold)
	require.Equal(t, 10, cfg.Guards.RejectionMinSamples)
	require.Equal(t, 0.02, cfg.Guards.FillDeviationPct)
	require.Equal(t, 12.5, cfg.Guards.TradesPerHourMean)
	require.Equal(t, 3.5, cfg.Guards.TradesPerHourStdDev)
	require.Equal(t, 8, cfg.Guards.TradesPerHourMinSamples)
	require.Equal(t, 90*time.Second, cfg.Guards.FeedStalenessThreshold)
	require.Equal(t, 10*time.Second, cfg.StalenessEvalInterval)
	require.True(t, cfg.RejectionEnabled)
	require.True(t, cfg.FillDeviationEnabled)
	require.True(t, cfg.TradesPerHourEnabled)
	require.True(t, cfg.FeedStalenessEnabled)
}

func TestLoadGuardEnvConfig_OffSentinelDisablesIndividualGuards(t *testing.T) {
	t.Setenv(EnvGuardRejectionThreshold, "off")
	t.Setenv(EnvGuardFillDeviationPct, "OFF") // case-insensitive
	t.Setenv(EnvGuardTradesPerHourMean, "off")
	t.Setenv(EnvGuardFeedStalenessThreshold, "off")

	cfg, err := LoadGuardEnvConfig()
	require.NoError(t, err)

	require.False(t, cfg.RejectionEnabled)
	require.False(t, cfg.FillDeviationEnabled)
	require.False(t, cfg.TradesPerHourEnabled)
	require.False(t, cfg.FeedStalenessEnabled)
}

func TestLoadGuardEnvConfig_OffOnOneGuardLeavesOthersEnabled(t *testing.T) {
	t.Setenv(EnvGuardFillDeviationPct, "off")

	cfg, err := LoadGuardEnvConfig()
	require.NoError(t, err)

	require.False(t, cfg.FillDeviationEnabled)
	require.True(t, cfg.RejectionEnabled)
	require.True(t, cfg.TradesPerHourEnabled)
	require.True(t, cfg.FeedStalenessEnabled)
}

func TestLoadGuardEnvConfig_InvalidValuesRefuseStartup(t *testing.T) {
	cases := []struct {
		name  string
		env   string
		value string
	}{
		{name: "bad duration", env: EnvGuardRejectionWindow, value: "ten minutes"},
		{name: "bad float", env: EnvGuardRejectionThreshold, value: "half"},
		{name: "bad int", env: EnvGuardRejectionMinSamples, value: "3.5"},
		{name: "bad deviation", env: EnvGuardFillDeviationPct, value: "5%"},
		{name: "bad mean", env: EnvGuardTradesPerHourMean, value: "many"},
		{name: "bad staleness threshold", env: EnvGuardFeedStalenessThreshold, value: "5 mins"},
		{name: "bad eval interval", env: EnvGuardStalenessEvalInterval, value: "soonish"},
		{name: "out-of-range rejection threshold", env: EnvGuardRejectionThreshold, value: "1.5"},
		{name: "negative deviation", env: EnvGuardFillDeviationPct, value: "-0.05"},
		{name: "zero min samples", env: EnvGuardRejectionMinSamples, value: "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.env, tc.value)
			_, err := LoadGuardEnvConfig()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.env, "the error must name the offending variable")
		})
	}
}

func TestBuildGuardRegistry_DisabledGuardIsNilAndOthersOperate(t *testing.T) {
	c := newTestController(t)
	cfg := GuardEnvConfig{
		Guards: GuardConfig{
			RejectionWindow:         time.Minute,
			RejectionThreshold:      0.5,
			RejectionMinSamples:     2,
			FillDeviationPct:        0.02,
			TradesPerHourMean:       3,
			TradesPerHourStdDev:     1,
			TradesPerHourMinSamples: 1,
			FeedStalenessThreshold:  30 * time.Second,
		},
		RejectionEnabled:     true,
		FillDeviationEnabled: false, // disabled via sentinel
		TradesPerHourEnabled: true,
		FeedStalenessEnabled: true,
	}

	reg := BuildGuardRegistry(c, NewFakeClock(time.Now()), cfg, nil)

	require.Nil(t, reg.FillDeviation, "a disabled guard must not be constructed")
	require.NotNil(t, reg.RejectionRate)
	require.NotNil(t, reg.TradesPerHour)
	require.NotNil(t, reg.FeedStaleness)

	// The remaining guards operate normally: two rejections trip.
	reg.RejectionRate.Observe(true)
	tripped, _ := reg.RejectionRate.Observe(true)
	require.True(t, tripped)
	require.True(t, c.Status().Engaged)
}
