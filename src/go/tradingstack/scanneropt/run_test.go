package scanneropt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// runSyntheticCycle runs one full cycle over the synthetic fixture with
// in-memory stores.
func runSyntheticCycle(t *testing.T) (*CycleResult, *overfitting.FakeVerdictStore, *FakeProposalStore) {
	t.Helper()
	verdicts := overfitting.NewFakeVerdictStore()
	proposals := NewFakeProposalStore()

	result, err := RunCycle(SyntheticWeightedRows(), SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err)
	return result, verdicts, proposals
}

// TestRunCycle_SyntheticPassesGateAndLandsPendingReview: the healthy
// stationary synthetic fixture clears all four gate checks under the default
// config and the proposal lands in the review queue.
func TestRunCycle_SyntheticPassesGateAndLandsPendingReview(t *testing.T) {
	result, verdicts, proposals := runSyntheticCycle(t)

	require.True(t, result.Verdict.Passed, "synthetic fixture must pass the gate; checks: %+v", result.Verdict.Checks)
	for _, c := range result.Verdict.Checks {
		assert.True(t, c.Passed, "check %s must pass: %s", c.Name, c.Detail)
	}

	assert.Equal(t, StatusPendingReview, result.Proposal.Status)
	assert.Equal(t, result.VerdictRow.ID, result.Proposal.VerdictID,
		"the proposal must reference the persisted verdict row")

	persisted, err := verdicts.FetchLatestByProposal(result.Proposal.ID)
	require.NoError(t, err)
	assert.True(t, persisted.Passed)

	listed, err := proposals.List()
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, result.Proposal.ID, listed[0].ID)
}

// TestRunCycle_SyntheticPayloadShape: the healthy regime derives a model, the
// thin regime does not, and the payload round-trips through scannercfg.
func TestRunCycle_SyntheticPayloadShape(t *testing.T) {
	result, _, _ := runSyntheticCycle(t)

	payload, err := scannercfg.Parse(result.Proposal.ProposedConfigJSON)
	require.NoError(t, err)
	require.NoError(t, payload.Validate())

	trending, ok := payload.RegimeModels["trending"]
	require.True(t, ok, "the healthy regime must derive a model")
	require.NotNil(t, trending.HardFilterOverrides.VolumeRatioFloor)
	require.NotNil(t, trending.HardFilterOverrides.ATRPctCeiling)
	require.Len(t, trending.FeatureWeights, 3)

	weightSum := 0.0
	for _, w := range trending.FeatureWeights {
		weightSum += w
	}
	assert.InDelta(t, 1.0, weightSum, 1e-9)

	_, hasThin := payload.RegimeModels["mean_reverting"]
	assert.False(t, hasThin, "the thin regime has no baseline model to carry and derives none")

	assert.Equal(t, "synthetic-proposal-v1", payload.Version)
}

// TestRunCycle_TruncatedRowsRejectedByGate: too few rows fail the
// minimum-sample gate; the proposal is persisted as rejected_by_gate and the
// failing verdict is persisted too.
func TestRunCycle_TruncatedRowsRejectedByGate(t *testing.T) {
	verdicts := overfitting.NewFakeVerdictStore()
	proposals := NewFakeProposalStore()

	rows := SyntheticWeightedRows()[:100]
	result, err := RunCycle(rows, SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err, "a gate rejection is a normal outcome, not an error")

	assert.False(t, result.Verdict.Passed)
	assert.Equal(t, StatusRejectedByGate, result.Proposal.Status)

	persisted, err := verdicts.FetchLatestByProposal(result.Proposal.ID)
	require.NoError(t, err)
	assert.False(t, persisted.Passed, "the failing verdict must be persisted as audit evidence")
	assert.Equal(t, result.Proposal.VerdictID, persisted.ID)
}

// TestRunCycle_EmptyRowsError: nothing to tune is an error, not an empty
// proposal.
func TestRunCycle_EmptyRowsError(t *testing.T) {
	_, err := RunCycle(nil, SyntheticBaseline(), SyntheticCycleConfig(), overfitting.NewFakeVerdictStore(), NewFakeProposalStore())
	assert.ErrorIs(t, err, ErrNoTrainingRows)
}

// TestRunCycle_NeverWritesScannerConfigs: an optimizer run -- pass or fail --
// creates no scanner_configs row. Only Promote does.
func TestRunCycle_NeverWritesScannerConfigs(t *testing.T) {
	_, _, proposals := runSyntheticCycle(t)
	assert.Empty(t, proposals.Configs(), "RunCycle must never write to scanner_configs")
}

// TestRunCycle_EvidenceJSONRoundTrips: the persisted evidence_json decodes
// back to the submitted evidence.
func TestRunCycle_EvidenceJSONRoundTrips(t *testing.T) {
	result, _, _ := runSyntheticCycle(t)

	decoded, err := UnmarshalEvidence(result.Proposal.EvidenceJSON)
	require.NoError(t, err)
	assert.Equal(t, result.Evidence, decoded)
}

// --- Fake store promote semantics (mirrored by the testcontainers suite) ---

func TestFakeProposalStore_PromotePendingCreatesVerbatimConfigAndFlipsStatus(t *testing.T) {
	result, _, proposals := runSyntheticCycle(t)
	require.Equal(t, StatusPendingReview, result.Proposal.Status)

	promotedAt := time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC)
	cfg, err := proposals.Promote(result.Proposal.ID, promotedAt)
	require.NoError(t, err)

	assert.Equal(t, string(result.Proposal.ProposedConfigJSON), string(cfg.ConfigJSON),
		"the promoted config_json must equal the proposal payload verbatim")
	require.NotNil(t, cfg.OptimizerRunID)
	assert.Equal(t, result.Proposal.OptimizerRunID, *cfg.OptimizerRunID)
	assert.Nil(t, cfg.Regime, "the payload is multi-regime; regime stays NULL")

	stored, err := proposals.FetchByID(result.Proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPromoted, stored.Status)
}

func TestFakeProposalStore_PromoteRefusesRejectedAndAlreadyPromoted(t *testing.T) {
	verdicts := overfitting.NewFakeVerdictStore()
	proposals := NewFakeProposalStore()

	rejected, err := RunCycle(SyntheticWeightedRows()[:100], SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err)
	require.Equal(t, StatusRejectedByGate, rejected.Proposal.Status)

	_, err = proposals.Promote(rejected.Proposal.ID, time.Now().UTC())
	assert.ErrorIs(t, err, ErrProposalNotPending, "a gate-rejected proposal can never be applied")
	assert.Empty(t, proposals.Configs())

	passing, err := RunCycle(SyntheticWeightedRows(), SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err)
	_, err = proposals.Promote(passing.Proposal.ID, time.Now().UTC())
	require.NoError(t, err)

	_, err = proposals.Promote(passing.Proposal.ID, time.Now().UTC())
	assert.ErrorIs(t, err, ErrProposalNotPending, "a proposal cannot be promoted twice")
	assert.Len(t, proposals.Configs(), 1)
}

func TestFakeProposalStore_PromoteUnknownIDNotFound(t *testing.T) {
	proposals := NewFakeProposalStore()
	_, err := proposals.Promote(uuid.New(), time.Now().UTC())
	assert.ErrorIs(t, err, ErrProposalNotFound)
}
