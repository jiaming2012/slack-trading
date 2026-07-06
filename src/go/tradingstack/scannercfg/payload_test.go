package scannercfg

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archDocExample is the architecture doc's Scanner Config Payload example
// verbatim (todo/full-trading-stack-architecture.md, "Scanner Config Payload
// (Optimizer Output)").
const archDocExample = `{
  "version": "2026-04-03T06:00:00Z",
  "regime_models": {
    "trending": {
      "feature_weights": {
        "volume_ratio": 0.31,
        "price_vs_50ma": 0.28,
        "rsi_14": 0.12,
        "short_interest": 0.09,
        "compression_score": 0.08,
        "sector_momentum": 0.12
      },
      "drop_features": ["pe_ratio"],
      "hard_filter_overrides": {
        "volume_ratio_floor": 1.2,
        "atr_pct_ceiling": 3.5
      },
      "score_threshold": 0.65
    },
    "high_vol": {
      "hard_filter_overrides": {
        "volume_ratio_floor": 1.8,
        "atr_pct_ceiling": 2.8
      },
      "score_threshold": 0.72
    }
  },
  "global": {
    "top_n_candidates": 20,
    "min_labeled_samples": 500
  }
}`

// TestParseMarshal_ArchDocExampleRoundTripsKeyForKey pins the semantic
// round-trip requirement: parsing the architecture doc's example payload and
// re-marshalling it yields a document with exactly the same keys and values.
func TestParseMarshal_ArchDocExampleRoundTripsKeyForKey(t *testing.T) {
	payload, err := Parse([]byte(archDocExample))
	require.NoError(t, err)

	remarshalled, err := payload.Marshal()
	require.NoError(t, err)

	var original, roundTripped map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(archDocExample), &original))
	require.NoError(t, json.Unmarshal(remarshalled, &roundTripped))

	assert.Equal(t, original, roundTripped,
		"re-marshalled payload must deserialize to the same keys and values as the original document")
}

// TestParse_FieldsMapFaithfully asserts the parsed struct carries the example
// document's values in the right fields.
func TestParse_FieldsMapFaithfully(t *testing.T) {
	payload, err := Parse([]byte(archDocExample))
	require.NoError(t, err)

	assert.Equal(t, "2026-04-03T06:00:00Z", payload.Version)
	require.Contains(t, payload.RegimeModels, "trending")
	require.Contains(t, payload.RegimeModels, "high_vol")

	trending := payload.RegimeModels["trending"]
	assert.Equal(t, 0.31, trending.FeatureWeights["volume_ratio"])
	assert.Equal(t, []string{"pe_ratio"}, trending.DropFeatures)
	require.NotNil(t, trending.HardFilterOverrides.VolumeRatioFloor)
	assert.Equal(t, 1.2, *trending.HardFilterOverrides.VolumeRatioFloor)
	require.NotNil(t, trending.HardFilterOverrides.ATRPctCeiling)
	assert.Equal(t, 3.5, *trending.HardFilterOverrides.ATRPctCeiling)
	require.NotNil(t, trending.ScoreThreshold)
	assert.Equal(t, 0.65, *trending.ScoreThreshold)

	highVol := payload.RegimeModels["high_vol"]
	assert.Nil(t, highVol.FeatureWeights, "high_vol carries no feature_weights in the example")
	assert.Nil(t, highVol.DropFeatures)
	require.NotNil(t, highVol.HardFilterOverrides.VolumeRatioFloor)
	assert.Equal(t, 1.8, *highVol.HardFilterOverrides.VolumeRatioFloor)

	assert.Equal(t, 20, payload.Global.TopNCandidates)
	assert.Equal(t, 500, payload.Global.MinLabeledSamples)
}

// TestParse_UnknownFieldRejected: a key this type cannot represent (and so
// could not round-trip) fails loudly at parse time.
func TestParse_UnknownFieldRejected(t *testing.T) {
	doc := `{"version": "v1", "regime_models": {}, "global": {"top_n_candidates": 20, "min_labeled_samples": 500}, "mystery": 1}`

	_, err := Parse([]byte(doc))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mystery")
}

func TestValidate_NegativeFeatureWeightRejected(t *testing.T) {
	payload, err := Parse([]byte(archDocExample))
	require.NoError(t, err)
	payload.RegimeModels["trending"].FeatureWeights["rsi_14"] = -0.1

	err = payload.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `feature weight "rsi_14"`)
	assert.Contains(t, err.Error(), `"trending"`)
}

func TestValidate_OutOfRangeScoreThresholdRejected(t *testing.T) {
	payload, err := Parse([]byte(archDocExample))
	require.NoError(t, err)

	bad := 1.5
	model := payload.RegimeModels["high_vol"]
	model.ScoreThreshold = &bad
	payload.RegimeModels["high_vol"] = model

	err = payload.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "score_threshold")
	assert.Contains(t, err.Error(), `"high_vol"`)
}

func TestValidate_ArchDocExamplePasses(t *testing.T) {
	payload, err := Parse([]byte(archDocExample))
	require.NoError(t, err)

	assert.NoError(t, payload.Validate())
}

func TestDefaultPayload_Validates(t *testing.T) {
	assert.NoError(t, DefaultPayload().Validate())
}

// TestClone_IsDeepAndIndependent guards the tuner's carry-forward path: a
// cloned payload shares no mutable state with its source.
func TestClone_IsDeepAndIndependent(t *testing.T) {
	payload, err := Parse([]byte(archDocExample))
	require.NoError(t, err)

	clone := payload.Clone()
	clone.RegimeModels["trending"].FeatureWeights["volume_ratio"] = 0.99
	*clone.RegimeModels["trending"].HardFilterOverrides.VolumeRatioFloor = 9.9
	clone.RegimeModels["trending"].DropFeatures[0] = "mutated"

	assert.Equal(t, 0.31, payload.RegimeModels["trending"].FeatureWeights["volume_ratio"])
	assert.Equal(t, 1.2, *payload.RegimeModels["trending"].HardFilterOverrides.VolumeRatioFloor)
	assert.Equal(t, []string{"pe_ratio"}, payload.RegimeModels["trending"].DropFeatures)
}
