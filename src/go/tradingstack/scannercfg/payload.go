// Package scannercfg owns the Go representation of the architecture doc's
// Scanner Config Payload: per-regime feature weights, dropped features, hard
// filter overrides, and score thresholds, plus a global section. It is
// deliberately dependency-light -- the scanner-optimizer (scanneropt) produces
// these payloads and the upcoming shadow-config-deployment change consumes
// them without importing the tuner.
//
// Payload JSON produced by Marshal is storable unchanged in the existing
// scanner_configs.config_json JSONB column.
package scannercfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

// Payload is the full Scanner Config Payload document.
type Payload struct {
	// Version identifies the payload version (the architecture doc uses an
	// RFC 3339 timestamp string).
	Version string `json:"version"`

	// RegimeModels maps a regime tag (e.g. "trending", "high_vol") to its
	// per-regime model.
	RegimeModels map[string]RegimeModel `json:"regime_models"`

	// Global carries the regime-independent settings.
	Global GlobalConfig `json:"global"`
}

// RegimeModel is one regime's tuned configuration. FeatureWeights and
// DropFeatures are omitted from JSON when empty (matching the architecture
// doc's "high_vol" example, which carries only overrides and a threshold).
type RegimeModel struct {
	// FeatureWeights maps a feature name to its non-negative weight.
	FeatureWeights map[string]float64 `json:"feature_weights,omitempty"`

	// DropFeatures lists feature names excluded from scoring.
	DropFeatures []string `json:"drop_features,omitempty"`

	// HardFilterOverrides tightens the scanner's hard filters for this
	// regime. Marshal always emits the key; a source document without it
	// parses to the zero value (no overrides) and re-marshals as an empty
	// object -- semantically identical.
	HardFilterOverrides HardFilterOverrides `json:"hard_filter_overrides"`

	// ScoreThreshold is the optional Layer 3 score cutoff in [0, 1].
	ScoreThreshold *float64 `json:"score_threshold,omitempty"`
}

// HardFilterOverrides carries the two tunable hard-filter bounds. Each is
// optional: a nil field imposes no override.
type HardFilterOverrides struct {
	VolumeRatioFloor *float64 `json:"volume_ratio_floor,omitempty"`
	ATRPctCeiling    *float64 `json:"atr_pct_ceiling,omitempty"`
}

// GlobalConfig is the payload's regime-independent section.
type GlobalConfig struct {
	// TopNCandidates is the number of candidates the scanner surfaces per
	// cycle.
	TopNCandidates int `json:"top_n_candidates"`

	// MinLabeledSamples is the minimum count of decided labeled samples a
	// regime must have before the optimizer derives a model for it.
	MinLabeledSamples int `json:"min_labeled_samples"`
}

// Parse decodes a payload JSON document. Unknown fields are rejected so a
// document whose keys this type cannot represent -- and therefore could not
// round-trip -- fails loudly instead of silently dropping keys.
func Parse(data []byte) (Payload, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var p Payload
	if err := dec.Decode(&p); err != nil {
		return Payload{}, fmt.Errorf("scannercfg: parse failed: %w", err)
	}
	return p, nil
}

// Marshal encodes the payload as JSON with stable key order (encoding/json
// sorts map keys), so identical payloads always serialize identically.
func (p Payload) Marshal() ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("scannercfg: marshal failed: %w", err)
	}
	return data, nil
}

// Validate rejects payloads with negative or non-finite feature weights, a
// score_threshold outside [0, 1], non-finite or negative hard-filter
// overrides, or a non-positive global section. The returned error names the
// offending field.
func (p Payload) Validate() error {
	for regime, model := range p.RegimeModels {
		for feature, weight := range model.FeatureWeights {
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				return fmt.Errorf("scannercfg: regime %q feature weight %q is not finite (%v)", regime, feature, weight)
			}
			if weight < 0 {
				return fmt.Errorf("scannercfg: regime %q feature weight %q is negative (%v)", regime, feature, weight)
			}
		}

		if t := model.ScoreThreshold; t != nil {
			if math.IsNaN(*t) || math.IsInf(*t, 0) || *t < 0 || *t > 1 {
				return fmt.Errorf("scannercfg: regime %q score_threshold %v is outside [0, 1]", regime, *t)
			}
		}

		if f := model.HardFilterOverrides.VolumeRatioFloor; f != nil {
			if math.IsNaN(*f) || math.IsInf(*f, 0) || *f < 0 {
				return fmt.Errorf("scannercfg: regime %q volume_ratio_floor %v is not a finite non-negative value", regime, *f)
			}
		}
		if c := model.HardFilterOverrides.ATRPctCeiling; c != nil {
			if math.IsNaN(*c) || math.IsInf(*c, 0) || *c < 0 {
				return fmt.Errorf("scannercfg: regime %q atr_pct_ceiling %v is not a finite non-negative value", regime, *c)
			}
		}
	}

	if p.Global.TopNCandidates < 1 {
		return fmt.Errorf("scannercfg: global top_n_candidates %d must be at least 1", p.Global.TopNCandidates)
	}
	if p.Global.MinLabeledSamples < 1 {
		return fmt.Errorf("scannercfg: global min_labeled_samples %d must be at least 1", p.Global.MinLabeledSamples)
	}

	return nil
}

// Clone returns a deep copy of the payload, so tuning may derive a proposal
// from a baseline without mutating it.
func (p Payload) Clone() Payload {
	out := Payload{
		Version: p.Version,
		Global:  p.Global,
	}
	if p.RegimeModels != nil {
		out.RegimeModels = make(map[string]RegimeModel, len(p.RegimeModels))
		for regime, model := range p.RegimeModels {
			out.RegimeModels[regime] = model.clone()
		}
	}
	return out
}

func (m RegimeModel) clone() RegimeModel {
	out := RegimeModel{}
	if m.FeatureWeights != nil {
		out.FeatureWeights = make(map[string]float64, len(m.FeatureWeights))
		for k, v := range m.FeatureWeights {
			out.FeatureWeights[k] = v
		}
	}
	if m.DropFeatures != nil {
		out.DropFeatures = append([]string(nil), m.DropFeatures...)
	}
	if m.HardFilterOverrides.VolumeRatioFloor != nil {
		f := *m.HardFilterOverrides.VolumeRatioFloor
		out.HardFilterOverrides.VolumeRatioFloor = &f
	}
	if m.HardFilterOverrides.ATRPctCeiling != nil {
		c := *m.HardFilterOverrides.ATRPctCeiling
		out.HardFilterOverrides.ATRPctCeiling = &c
	}
	if m.ScoreThreshold != nil {
		t := *m.ScoreThreshold
		out.ScoreThreshold = &t
	}
	return out
}

// DefaultPayload returns the built-in baseline used when the scanner_configs
// table is empty: no per-regime models (before any optimizer run there is no
// tuned model -- the scanner's own hard-coded thresholds govern) and the
// architecture doc's global defaults (top 20 candidates, 500 minimum labeled
// samples).
func DefaultPayload() Payload {
	return Payload{
		Version:      "builtin-default-v1",
		RegimeModels: map[string]RegimeModel{},
		Global: GlobalConfig{
			TopNCandidates:    20,
			MinLabeledSamples: 500,
		},
	}
}
