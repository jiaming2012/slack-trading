package scanneropt

import (
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// sortRowsChronologically returns a copy of rows ordered by ScannedAt
// ascending, tie-broken by Ticker and then ScanResultID so identical inputs
// always sort identically.
func sortRowsChronologically(rows []optvalidation.WeightedTrainingRow) []optvalidation.WeightedTrainingRow {
	sorted := append([]optvalidation.WeightedTrainingRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if !a.ScannedAt.Equal(b.ScannedAt) {
			return a.ScannedAt.Before(b.ScannedAt)
		}
		if a.Ticker != b.Ticker {
			return a.Ticker < b.Ticker
		}
		return a.ScanResultID.String() < b.ScanResultID.String()
	})
	return sorted
}

// splitInOut splits chronologically sorted rows 80/20 into the derivation
// (in-sample) segment and the held-out (out-of-sample) tail.
func splitInOut(sorted []optvalidation.WeightedTrainingRow) (inSample, outOfSample []optvalidation.WeightedTrainingRow) {
	cut := len(sorted) * 4 / 5
	return sorted[:cut], sorted[cut:]
}

// BuildEvidence constructs the overfitting.Evidence for a proposed payload:
//
//   - inSample and outOfSample are the chronological 80/20 split of the
//     weighted rows; the proposal was derived from inSample only.
//   - The in-sample segment is split into minFolds+1 equal chronological
//     segments producing minFolds walk-forward folds: fold i trains on the
//     prefix (segments 1..i) and tests on segment i+1, re-deriving the
//     proposal parameters from the fold's train prefix (the fold's Params
//     carry that fold's derived volume_ratio_floor, atr_pct_ceiling, and
//     feature weights, feeding the gate's cross-fold stability check).
//   - The per-sample performance measure is EVWeight * PnlPct over rows the
//     proposed overrides would have admitted (volume_ratio >= floor AND
//     atr_pct <= ceiling for the row's regime; a regime with no model or no
//     override does not restrict admission).
//   - SampleSize is the count of decided (PnlPct != 0) rows with a non-empty
//     regime tag in the derivation segment -- the samples the proposal was
//     actually derived from.
//   - TrialsCount is 1: this is a direct deterministic derivation, not a
//     search.
//   - ProposedParams/BaselineParams are the flattened numeric knobs
//     (<regime>.volume_ratio_floor, <regime>.atr_pct_ceiling,
//     <regime>.feature_weight.<feature>) of the proposed and baseline
//     payloads.
func BuildEvidence(
	inSample, outOfSample []optvalidation.WeightedTrainingRow,
	baseline, proposed scannercfg.Payload,
	proposalID uuid.UUID,
	minFolds int,
) (overfitting.Evidence, error) {
	if minFolds < 1 {
		return overfitting.Evidence{}, fmt.Errorf("%w: minimum fold count %d must be at least 1", ErrInsufficientRows, minFolds)
	}
	segmentCount := minFolds + 1
	if len(inSample) < segmentCount || len(outOfSample) == 0 {
		return overfitting.Evidence{}, fmt.Errorf("%w: %d in-sample rows for %d fold segments, %d out-of-sample rows",
			ErrInsufficientRows, len(inSample), segmentCount, len(outOfSample))
	}

	// --- Walk-forward folds over the in-sample segment ---
	segments := splitSegments(inSample, segmentCount)
	folds := make([]overfitting.FoldResult, 0, minFolds)
	for i := 1; i <= minFolds; i++ {
		trainPrefix := inSample[:segmentStart(len(inSample), segmentCount, i)]
		testSegment := segments[i]

		foldPayload, err := Tune(trainPrefix, baseline, fmt.Sprintf("fold-%d", i))
		if err != nil {
			return overfitting.Evidence{}, fmt.Errorf("scanneropt: fold %d re-derivation failed: %w", i, err)
		}

		testMeasures := admittedMeasures(testSegment, foldPayload)
		testMetric, _ := meanStdDev(testMeasures)

		folds = append(folds, overfitting.FoldResult{
			Index:      i,
			TrainStart: trainPrefix[0].ScannedAt,
			TrainEnd:   trainPrefix[len(trainPrefix)-1].ScannedAt,
			TestStart:  testSegment[0].ScannedAt,
			TestEnd:    testSegment[len(testSegment)-1].ScannedAt,
			Params:     flattenParams(foldPayload),
			TestMetric: testMetric,
		})
	}

	// --- Performance over the derivation and holdout windows ---
	inMeasures := admittedMeasures(inSample, proposed)
	inMean, inStdDev := meanStdDev(inMeasures)
	oosMeasures := admittedMeasures(outOfSample, proposed)
	oosMean, oosStdDev := meanStdDev(oosMeasures)

	return overfitting.Evidence{
		ProposalID:   proposalID,
		ProposalKind: overfitting.ProposalKindScanner,
		SampleSize:   countDecided(inSample),
		TrialsCount:  1,
		InSample: overfitting.Performance{
			WindowStart: inSample[0].ScannedAt,
			WindowEnd:   inSample[len(inSample)-1].ScannedAt,
			Mean:        inMean,
			StdDev:      inStdDev,
			SampleSize:  len(inMeasures),
		},
		OutOfSample: overfitting.Performance{
			WindowStart: outOfSample[0].ScannedAt,
			WindowEnd:   outOfSample[len(outOfSample)-1].ScannedAt,
			Mean:        oosMean,
			StdDev:      oosStdDev,
			SampleSize:  len(oosMeasures),
		},
		Folds:          folds,
		ProposedParams: flattenParams(proposed),
		BaselineParams: flattenParams(baseline),
	}, nil
}

// segmentStart returns the start index of segment i (0-based over
// segmentCount equal chronological segments of n rows).
func segmentStart(n, segmentCount, i int) int {
	return n * i / segmentCount
}

// splitSegments splits sorted rows into segmentCount contiguous chronological
// segments of (near-)equal size.
func splitSegments(rows []optvalidation.WeightedTrainingRow, segmentCount int) [][]optvalidation.WeightedTrainingRow {
	segments := make([][]optvalidation.WeightedTrainingRow, segmentCount)
	for i := 0; i < segmentCount; i++ {
		start := segmentStart(len(rows), segmentCount, i)
		end := segmentStart(len(rows), segmentCount, i+1)
		segments[i] = rows[start:end]
	}
	return segments
}

// admittedMeasures returns the per-sample performance measure
// (EVWeight * PnlPct) of every row the payload's hard-filter overrides would
// have admitted. A row's regime with no model in the payload, or a model
// without an override, does not restrict admission.
func admittedMeasures(rows []optvalidation.WeightedTrainingRow, payload scannercfg.Payload) []float64 {
	measures := make([]float64, 0, len(rows))
	for _, r := range rows {
		if model, ok := payload.RegimeModels[r.RegimeTag]; ok {
			if f := model.HardFilterOverrides.VolumeRatioFloor; f != nil && r.VolumeRatio < *f {
				continue
			}
			if c := model.HardFilterOverrides.ATRPctCeiling; c != nil && r.ATRPct > *c {
				continue
			}
		}
		measures = append(measures, r.EVWeight*r.PnlPct)
	}
	return measures
}

// countDecided counts rows with a decided outcome (PnlPct != 0) and a
// non-empty regime tag -- the samples the tuner derives from.
func countDecided(rows []optvalidation.WeightedTrainingRow) int {
	count := 0
	for _, r := range rows {
		if r.RegimeTag != "" && r.PnlPct != 0 {
			count++
		}
	}
	return count
}

// flattenParams flattens a payload's comparable numeric knobs into the flat
// parameter map the gate's stability check consumes:
// <regime>.volume_ratio_floor, <regime>.atr_pct_ceiling, and
// <regime>.feature_weight.<feature>.
func flattenParams(payload scannercfg.Payload) map[string]float64 {
	params := make(map[string]float64)
	for regime, model := range payload.RegimeModels {
		if f := model.HardFilterOverrides.VolumeRatioFloor; f != nil {
			params[regime+".volume_ratio_floor"] = *f
		}
		if c := model.HardFilterOverrides.ATRPctCeiling; c != nil {
			params[regime+".atr_pct_ceiling"] = *c
		}
		for feature, weight := range model.FeatureWeights {
			params[regime+".feature_weight."+feature] = weight
		}
	}
	return params
}
