package shadowdeploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scanneropt"
)

// ReplayLimitation is the structural limitation of shadow-by-replay, surfaced
// in every run report rather than hidden: observations are the scan_results
// rows the ACTIVE pipeline persisted, so a shadow config that would loosen
// Layer-1 admission cannot surface tickers that were never scanned. The named
// future changes scanner-config-hot-swap and shadow-outcome-backfill lift
// this.
const ReplayLimitation = "limitation: shadow evaluation replays persisted scan_results, so tickers the active pipeline never persisted are invisible -- a shadow config that would loosen Layer-1 admission cannot surface never-scanned tickers (lifted by the future scanner-config-hot-swap and shadow-outcome-backfill changes)"

// RunResult is everything one shadow run produced: the persisted run row and
// divergence rows, both payloads and decision vectors, the divergence report,
// and the coverage-explicit outcome comparison.
type RunResult struct {
	Run         ShadowRun
	Divergences []ShadowDivergence

	Proposal      scanneropt.ScannerConfigProposal
	ActivePayload scannercfg.Payload
	ShadowPayload scannercfg.Payload

	ActiveDecisions []Decision
	ShadowDecisions []Decision

	Report   DivergenceReport
	Outcomes OutcomeComparison
}

// RunShadow executes one shadow run in the half-open scanned_at window
// [from, to):
//
//  1. Simulation-only guard FIRST: any mode but Simulation returns
//     ErrNotSimulation before anything is loaded or written (db may even be
//     nil) -- shadow evidence can never be generated from a Paper or Margin
//     context.
//  2. The shadow candidate proposal is fetched; status rejected_by_gate is
//     refused with ErrProposalRejectedByGate (pending_review is the primary
//     use; promoted is accepted for post-promotion monitoring).
//  3. Observations are loaded ONCE from persisted scan_results rows and
//     passed to both evaluations unchanged, so both sides see byte-identical
//     inputs. An empty window returns ErrNoObservations and persists nothing.
//  4. The active payload (latest scanner_configs row via the shared
//     scanneropt.LoadActiveBaseline rule; built-in default with a NULL
//     active_config_id when the table is empty) and the shadow payload are
//     evaluated over the same slice; divergence and the coverage-explicit
//     outcome comparison are computed and persisted via the store.
//
// RunShadow reads scanner_config_proposals, scanner_configs, scan_results,
// and sim_outcomes, and writes ONLY shadow_runs and shadow_divergences. It
// places no orders and never changes a proposal's status.
func RunShadow(mode models.Mode, db *gorm.DB, store ShadowStore, proposalID uuid.UUID, from, to time.Time, now time.Time) (*RunResult, error) {
	if mode != models.ModeSimulation {
		return nil, fmt.Errorf("%w: got mode %q", ErrNotSimulation, mode)
	}

	proposal, err := scanneropt.NewGormProposalStore(db).FetchByID(proposalID)
	if err != nil {
		return nil, err
	}
	if proposal.Status == scanneropt.StatusRejectedByGate {
		return nil, fmt.Errorf("%w: proposal %s", ErrProposalRejectedByGate, proposalID)
	}

	shadowPayload, err := scannercfg.Parse(proposal.ProposedConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("RunShadow: proposal %s payload is unparseable: %w", proposalID, err)
	}
	if err := shadowPayload.Validate(); err != nil {
		return nil, fmt.Errorf("RunShadow: proposal %s payload is invalid: %w", proposalID, err)
	}

	activePayload, activeConfigID, err := loadActiveConfig(db)
	if err != nil {
		return nil, err
	}

	observations, err := LoadObservations(db, from, to)
	if err != nil {
		return nil, err
	}
	if len(observations) == 0 {
		return nil, fmt.Errorf("%w: window [%s, %s)", ErrNoObservations, from.Format(time.RFC3339), to.Format(time.RFC3339))
	}

	// Both payloads run over the identical observation slice: zero input
	// skew by construction.
	activeDecisions := EvaluateConfig(activePayload, observations)
	shadowDecisions := EvaluateConfig(shadowPayload, observations)

	report := CompareDecisions(activeDecisions, shadowDecisions)

	outcomesByScanResult, err := loadOutcomes(db, observations)
	if err != nil {
		return nil, err
	}
	outcomes := CompareOutcomes(activeDecisions, shadowDecisions, outcomesByScanResult)

	summaryJSON, err := json.Marshal(outcomes)
	if err != nil {
		return nil, fmt.Errorf("RunShadow: outcome summary marshal failed: %w", err)
	}

	run := ShadowRun{
		ID:                 uuid.New(),
		CreatedAt:          now,
		ActiveConfigID:     activeConfigID,
		ProposalID:         proposal.ID,
		WindowStart:        from,
		WindowEnd:          to,
		TotalObservations:  report.TotalObservations,
		SelectedActive:     report.SelectedActive,
		SelectedShadow:     report.SelectedShadow,
		SelectedBoth:       report.SelectedBoth,
		DivergencePct:      report.DivergencePct,
		OutcomeSummaryJSON: summaryJSON,
		Synthetic:          false,
	}
	divergences := DivergencesFromReport(run.ID, report)

	if err := store.PersistRun(run, divergences); err != nil {
		return nil, err
	}

	return &RunResult{
		Run:             run,
		Divergences:     divergences,
		Proposal:        *proposal,
		ActivePayload:   activePayload,
		ShadowPayload:   shadowPayload,
		ActiveDecisions: activeDecisions,
		ShadowDecisions: shadowDecisions,
		Report:          report,
		Outcomes:        outcomes,
	}, nil
}

// DivergencesFromReport flattens a divergence report into persistable rows
// (shadow_only then active_only, each tickers ascending as the report pins).
func DivergencesFromReport(runID uuid.UUID, report DivergenceReport) []ShadowDivergence {
	rows := make([]ShadowDivergence, 0, len(report.ShadowOnly)+len(report.ActiveOnly))
	for _, d := range report.ShadowOnly {
		rows = append(rows, ShadowDivergence{
			ShadowRunID:  runID,
			Ticker:       d.Ticker,
			Kind:         KindShadowOnly,
			ActiveScore:  d.ActiveScore,
			ShadowScore:  d.ShadowScore,
			ScanResultID: d.ScanResultID,
		})
	}
	for _, d := range report.ActiveOnly {
		rows = append(rows, ShadowDivergence{
			ShadowRunID:  runID,
			Ticker:       d.Ticker,
			Kind:         KindActiveOnly,
			ActiveScore:  d.ActiveScore,
			ShadowScore:  d.ShadowScore,
			ScanResultID: d.ScanResultID,
		})
	}
	return rows
}

// LoadObservations loads the scan_results rows whose scanned_at falls in the
// half-open window [from, to), ordered by scanned_at then id so identical
// database states load identically, lifted into engine observations.
func LoadObservations(db *gorm.DB, from, to time.Time) ([]Observation, error) {
	var scanResults []tradingstack.ScanResult
	if err := db.
		Where("scanned_at >= ? AND scanned_at < ?", from, to).
		Order("scanned_at ASC, id ASC").
		Find(&scanResults).Error; err != nil {
		return nil, fmt.Errorf("LoadObservations: scan_results query failed: %w", err)
	}

	observations := make([]Observation, len(scanResults))
	for i, sr := range scanResults {
		observations[i] = ObservationFromScanResult(sr)
	}
	return observations, nil
}

// loadOutcomes loads the existing sim_outcomes rows for the observations'
// scan result ids, keyed for the outcome comparison join. Only outcomes that
// already exist are loaded -- nothing is simulated or imputed here.
func loadOutcomes(db *gorm.DB, observations []Observation) (map[uuid.UUID][]tradingstack.SimOutcome, error) {
	if len(observations) == 0 {
		return map[uuid.UUID][]tradingstack.SimOutcome{}, nil
	}

	ids := make([]uuid.UUID, len(observations))
	for i, o := range observations {
		ids[i] = o.ScanResultID
	}

	var outcomes []tradingstack.SimOutcome
	if err := db.
		Where("scan_result_id IN ?", ids).
		Order("simulated_at ASC, id ASC").
		Find(&outcomes).Error; err != nil {
		return nil, fmt.Errorf("loadOutcomes: sim_outcomes query failed: %w", err)
	}

	byScanResult := make(map[uuid.UUID][]tradingstack.SimOutcome, len(outcomes))
	for _, o := range outcomes {
		byScanResult[o.ScanResultID] = append(byScanResult[o.ScanResultID], o)
	}
	return byScanResult, nil
}

// loadActiveConfig resolves the active baseline payload via the shared
// scanneropt.LoadActiveBaseline rule (latest scanner_configs row by
// created_at, else the built-in default) plus the matching row id -- NULL
// when the table was empty and the default payload is the baseline.
func loadActiveConfig(db *gorm.DB) (scannercfg.Payload, *uuid.UUID, error) {
	payload, err := scanneropt.LoadActiveBaseline(db)
	if err != nil {
		return scannercfg.Payload{}, nil, err
	}
	if err := payload.Validate(); err != nil {
		return scannercfg.Payload{}, nil, fmt.Errorf("loadActiveConfig: active payload is invalid: %w", err)
	}

	// Same ordering rule as LoadActiveBaseline, to recover the row id.
	var cfg tradingstack.ScannerConfig
	lookupErr := db.Order("created_at DESC NULLS LAST, id ASC").First(&cfg).Error
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return payload, nil, nil
	}
	if lookupErr != nil {
		return scannercfg.Payload{}, nil, fmt.Errorf("loadActiveConfig: scanner_configs lookup failed: %w", lookupErr)
	}

	id := cfg.ID
	return payload, &id, nil
}
