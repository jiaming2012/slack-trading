package scanneropt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// CycleConfig parameterizes one optimizer cycle. All clock values are
// caller-supplied so a cycle is a deterministic function of its inputs.
type CycleConfig struct {
	// Now stamps the verdict's computed_at and the proposal's created_at.
	Now time.Time

	// RunID identifies the optimizer run (carried onto the proposal and,
	// on promotion, onto the scanner_configs row).
	RunID string

	// Version is the proposed payload's version string.
	Version string

	// GateConfig carries the overfitting gate thresholds.
	GateConfig overfitting.Config
}

// CycleResult is everything one optimizer cycle produced: the proposed
// payload, the evidence submitted to the gate, the verdict, and the persisted
// proposal row (whose Status records the gate outcome).
type CycleResult struct {
	Payload    scannercfg.Payload
	Evidence   overfitting.Evidence
	Verdict    overfitting.Verdict
	VerdictRow overfitting.OverfittingVerdict
	Proposal   ScannerConfigProposal
}

// evidenceRecord is the JSON shape of a proposal row's evidence_json column
// (overfitting.Evidence's top-level fields carry no json tags of their own).
type evidenceRecord struct {
	ProposalID     uuid.UUID                `json:"proposal_id"`
	ProposalKind   string                   `json:"proposal_kind"`
	SampleSize     int                      `json:"sample_size"`
	TrialsCount    int                      `json:"trials_count"`
	InSample       overfitting.Performance  `json:"in_sample"`
	OutOfSample    overfitting.Performance  `json:"out_of_sample"`
	Folds          []overfitting.FoldResult `json:"folds"`
	ProposedParams map[string]float64       `json:"proposed_params"`
	BaselineParams map[string]float64       `json:"baseline_params"`
}

// marshalEvidence serializes gate evidence for the proposal row.
func marshalEvidence(e overfitting.Evidence) ([]byte, error) {
	record := evidenceRecord{
		ProposalID:     e.ProposalID,
		ProposalKind:   string(e.ProposalKind),
		SampleSize:     e.SampleSize,
		TrialsCount:    e.TrialsCount,
		InSample:       e.InSample,
		OutOfSample:    e.OutOfSample,
		Folds:          e.Folds,
		ProposedParams: e.ProposedParams,
		BaselineParams: e.BaselineParams,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("scanneropt: evidence marshal failed: %w", err)
	}
	return data, nil
}

// UnmarshalEvidence decodes a proposal row's evidence_json back into the gate
// evidence shape (used by the operator list command's evidence summary).
func UnmarshalEvidence(data []byte) (overfitting.Evidence, error) {
	var record evidenceRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return overfitting.Evidence{}, fmt.Errorf("scanneropt: evidence unmarshal failed: %w", err)
	}
	return overfitting.Evidence{
		ProposalID:     record.ProposalID,
		ProposalKind:   overfitting.ProposalKind(record.ProposalKind),
		SampleSize:     record.SampleSize,
		TrialsCount:    record.TrialsCount,
		InSample:       record.InSample,
		OutOfSample:    record.OutOfSample,
		Folds:          record.Folds,
		ProposedParams: record.ProposedParams,
		BaselineParams: record.BaselineParams,
	}, nil
}

// RunCycle executes one full optimizer cycle over the clean weighted training
// rows (the optimizer-validation-pipeline's output -- the ONLY training input
// this package accepts):
//
//  1. Order rows chronologically and hold out the final 20%.
//  2. Derive the proposed payload from the derivation segment only.
//  3. Build walk-forward evidence and submit it to the overfitting gate.
//  4. Persist the verdict -- pass or fail -- via the verdict store.
//  5. Persist the proposal with status pending_review (gate passed) or
//     rejected_by_gate (gate failed).
//
// RunCycle NEVER writes to scanner_configs: promotion is a separate,
// operator-only operation (ProposalStore.Promote).
func RunCycle(
	rows []optvalidation.WeightedTrainingRow,
	baseline scannercfg.Payload,
	cfg CycleConfig,
	verdicts overfitting.VerdictStore,
	proposals ProposalStore,
) (*CycleResult, error) {
	if len(rows) == 0 {
		return nil, ErrNoTrainingRows
	}

	sorted := sortRowsChronologically(rows)
	inSample, outOfSample := splitInOut(sorted)

	proposalID := uuid.New()

	payload, err := Tune(inSample, baseline, cfg.Version)
	if err != nil {
		return nil, err
	}

	evidence, err := BuildEvidence(inSample, outOfSample, baseline, payload, proposalID, cfg.GateConfig.MinFolds)
	if err != nil {
		return nil, err
	}

	verdict, verdictRow, err := SubmitToGate(evidence, cfg.GateConfig, cfg.Now, verdicts)
	if err != nil {
		return nil, err
	}

	status := StatusRejectedByGate
	if verdict.Passed {
		status = StatusPendingReview
	}

	payloadJSON, err := payload.Marshal()
	if err != nil {
		return nil, err
	}
	evidenceJSON, err := marshalEvidence(evidence)
	if err != nil {
		return nil, err
	}

	proposal := ScannerConfigProposal{
		ID:                 proposalID,
		CreatedAt:          cfg.Now,
		OptimizerRunID:     cfg.RunID,
		ProposedConfigJSON: payloadJSON,
		EvidenceJSON:       evidenceJSON,
		VerdictID:          verdictRow.ID,
		Status:             status,
	}
	if err := proposals.Persist(proposal); err != nil {
		return nil, err
	}

	return &CycleResult{
		Payload:    payload,
		Evidence:   evidence,
		Verdict:    verdict,
		VerdictRow: verdictRow,
		Proposal:   proposal,
	}, nil
}
