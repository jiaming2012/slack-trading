package scanneropt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// evidenceFixtureRows generates n deterministic hourly "trending" rows with a
// stationary win/loss pattern (two wins, then one loss): winners live at high
// volume_ratio and low atr_pct, losers at the opposite corner, so per-fold
// re-derivations land on near-identical parameters.
func evidenceFixtureRows(n int) []optvalidation.WeightedTrainingRow {
	base := time.Date(2024, 4, 1, 9, 0, 0, 0, time.UTC)
	rows := make([]optvalidation.WeightedTrainingRow, 0, n)
	winIdx := 0
	for i := 0; i < n; i++ {
		at := base.Add(time.Duration(i) * time.Hour)
		if i%3 == 2 {
			rows = append(rows, testRow("trending", at,
				-0.01, 1.0,
				40+float64(i%5),
				0.8+0.05*float64(i%4),
				4.0+0.1*float64(i%4)))
			continue
		}
		rows = append(rows, testRow("trending", at,
			0.015+0.005*float64(winIdx%3), 1.0,
			60+float64(winIdx%5),
			1.5+0.1*float64(winIdx%5),
			2.0+0.1*float64(winIdx%4)))
		winIdx++
	}
	return rows
}

// evidenceBaseline is a baseline payload with no regime models and a small
// min_labeled_samples so per-fold train prefixes derive.
func evidenceBaseline() scannercfg.Payload {
	return scannercfg.Payload{
		Version:      "base-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{},
		Global:       scannercfg.GlobalConfig{TopNCandidates: 20, MinLabeledSamples: 4},
	}
}

func buildFixtureEvidence(t *testing.T, n int) (overfitting.Evidence, []optvalidation.WeightedTrainingRow, []optvalidation.WeightedTrainingRow) {
	t.Helper()

	sorted := sortRowsChronologically(evidenceFixtureRows(n))
	inSample, outOfSample := splitInOut(sorted)

	proposed, err := Tune(inSample, evidenceBaseline(), "proposed-v1")
	require.NoError(t, err)

	evidence, err := BuildEvidence(inSample, outOfSample, evidenceBaseline(), proposed, uuid.New(), overfitting.DefaultConfig().MinFolds)
	require.NoError(t, err)
	return evidence, inSample, outOfSample
}

// TestBuildEvidence_FoldsChronologicalAndLookaheadFree pins the spec scenario:
// every fold's TestStart is at or after its TrainEnd, test windows are
// chronologically ordered, and the out-of-sample window starts after every
// fold's windows.
func TestBuildEvidence_FoldsChronologicalAndLookaheadFree(t *testing.T) {
	evidence, inSample, outOfSample := buildFixtureEvidence(t, 60)

	require.Len(t, evidence.Folds, overfitting.DefaultConfig().MinFolds)
	for i, fold := range evidence.Folds {
		assert.Equal(t, i+1, fold.Index, "fold indices must be strictly increasing from 1")
		assert.False(t, fold.TestStart.Before(fold.TrainEnd),
			"fold %d: test window must start at or after its train window ends", fold.Index)
		assert.True(t, fold.TrainStart.Equal(inSample[0].ScannedAt),
			"fold %d: training always starts at the beginning of the derivation window", fold.Index)
		if i > 0 {
			assert.False(t, fold.TestStart.Before(evidence.Folds[i-1].TestEnd),
				"fold %d: test windows must be chronologically ordered", fold.Index)
		}
		assert.False(t, evidence.OutOfSample.WindowStart.Before(fold.TestEnd),
			"the out-of-sample window must start after fold %d's windows", fold.Index)
	}

	assert.True(t, evidence.OutOfSample.WindowStart.Equal(outOfSample[0].ScannedAt))
	assert.True(t, evidence.InSample.WindowEnd.Before(evidence.OutOfSample.WindowStart),
		"holdout must be strictly after the derivation window in this fixture")
}

// TestBuildEvidence_FoldParamsCarryPerFoldDerivedValues: each fold's Params
// carry that fold's derived floor, ceiling, and feature weights.
func TestBuildEvidence_FoldParamsCarryPerFoldDerivedValues(t *testing.T) {
	evidence, _, _ := buildFixtureEvidence(t, 60)

	for _, fold := range evidence.Folds {
		assert.Contains(t, fold.Params, "trending.volume_ratio_floor", "fold %d", fold.Index)
		assert.Contains(t, fold.Params, "trending.atr_pct_ceiling", "fold %d", fold.Index)
		assert.Contains(t, fold.Params, "trending.feature_weight.rsi_14", "fold %d", fold.Index)
		assert.Contains(t, fold.Params, "trending.feature_weight.volume_ratio", "fold %d", fold.Index)
		assert.Contains(t, fold.Params, "trending.feature_weight.atr_pct", "fold %d", fold.Index)
	}
}

// TestBuildEvidence_HonestBookkeeping: TrialsCount is 1 (direct derivation,
// no search), SampleSize counts the decided derivation rows, and the
// performance windows cover only admitted rows.
func TestBuildEvidence_HonestBookkeeping(t *testing.T) {
	evidence, inSample, outOfSample := buildFixtureEvidence(t, 60)

	assert.Equal(t, 1, evidence.TrialsCount, "a direct derivation reports exactly 1 trial")
	assert.Equal(t, overfitting.ProposalKindScanner, evidence.ProposalKind)
	assert.Equal(t, countDecided(inSample), evidence.SampleSize,
		"SampleSize is the decided derivation-segment count")
	assert.Equal(t, len(inSample), 48, "80%% of 60 rows derive")
	assert.Equal(t, len(outOfSample), 12)

	// The fixture's losers sit below the winners' volume_ratio floor and
	// above their atr_pct ceiling, so the admitted count is strictly less
	// than the segment size but positive.
	assert.Greater(t, evidence.InSample.SampleSize, 0)
	assert.Less(t, evidence.InSample.SampleSize, len(inSample))
	assert.Greater(t, evidence.OutOfSample.SampleSize, 0)
	assert.Greater(t, evidence.InSample.Mean, 0.0, "admitted rows are dominated by winners")
	assert.Greater(t, evidence.OutOfSample.Mean, 0.0)
}

// TestBuildEvidence_Deterministic: identical inputs produce identical
// evidence.
func TestBuildEvidence_Deterministic(t *testing.T) {
	rows := evidenceFixtureRows(60)
	sorted := sortRowsChronologically(rows)
	inSample, outOfSample := splitInOut(sorted)
	proposed, err := Tune(inSample, evidenceBaseline(), "proposed-v1")
	require.NoError(t, err)
	proposalID := uuid.New()

	first, err := BuildEvidence(inSample, outOfSample, evidenceBaseline(), proposed, proposalID, 3)
	require.NoError(t, err)
	second, err := BuildEvidence(inSample, outOfSample, evidenceBaseline(), proposed, proposalID, 3)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

// TestBuildEvidence_InsufficientRowsErrors: a derivation segment too small to
// form the fold segments, or an empty holdout, is a structural error.
func TestBuildEvidence_InsufficientRowsErrors(t *testing.T) {
	rows := sortRowsChronologically(evidenceFixtureRows(4))
	proposed, err := Tune(rows, evidenceBaseline(), "proposed-v1")
	require.NoError(t, err)

	_, err = BuildEvidence(rows[:2], rows[2:], evidenceBaseline(), proposed, uuid.New(), 3)
	assert.ErrorIs(t, err, ErrInsufficientRows)

	_, err = BuildEvidence(rows, nil, evidenceBaseline(), proposed, uuid.New(), 3)
	assert.ErrorIs(t, err, ErrInsufficientRows)
}

// TestSubmitToGate_FailingVerdictStillPersisted pins the spec scenario: a
// failing gate run still persists an overfitting_verdicts row with passed
// false, referenced by the returned row's id.
func TestSubmitToGate_FailingVerdictStillPersisted(t *testing.T) {
	store := overfitting.NewFakeVerdictStore()
	evidence := overfitting.SyntheticEvidenceFail()
	computedAt := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)

	verdict, row, err := SubmitToGate(evidence, overfitting.DefaultConfig(), computedAt, store)
	require.NoError(t, err, "a failing verdict is not an error")

	assert.False(t, verdict.Passed)
	assert.NotEqual(t, uuid.Nil, row.ID, "the verdict row id is pre-assigned for the proposal FK")

	persisted, err := store.FetchLatestByProposal(evidence.ProposalID)
	require.NoError(t, err)
	assert.Equal(t, row.ID, persisted.ID)
	assert.False(t, persisted.Passed)
}

// TestSubmitToGate_PassingVerdictPersisted: the passing path persists too.
func TestSubmitToGate_PassingVerdictPersisted(t *testing.T) {
	store := overfitting.NewFakeVerdictStore()
	evidence := overfitting.SyntheticEvidencePass()

	verdict, row, err := SubmitToGate(evidence, overfitting.DefaultConfig(), time.Now().UTC(), store)
	require.NoError(t, err)

	assert.True(t, verdict.Passed)
	persisted, err := store.FetchLatestByProposal(evidence.ProposalID)
	require.NoError(t, err)
	assert.Equal(t, row.ID, persisted.ID)
	assert.True(t, persisted.Passed)
}
