package stratopt

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PersistRun writes one strategy_proposals row per proposal, stamping every
// row with the run's id. Each proposal's status must already reflect its gate
// verdict (pending_review on pass, rejected_by_gate on fail) and its
// VerdictID must reference a verdict persisted through the overfitting
// package's VerdictStore.
func PersistRun(db *gorm.DB, runID uuid.UUID, proposals []StrategyProposal) error {
	for i := range proposals {
		proposals[i].RunID = runID
		if proposals[i].VerdictID == uuid.Nil {
			return fmt.Errorf("stratopt: proposal %s has no verdict id", proposals[i].ID)
		}
		if err := db.Create(&proposals[i]).Error; err != nil {
			return fmt.Errorf("PersistRun: create failed for proposal %s: %w", proposals[i].ID, err)
		}
	}
	return nil
}

// ListProposals returns proposals filtered by status, newest first (ties
// broken by id for deterministic output). An empty statusFilter defaults to
// the pending_review review queue -- rejected_by_gate rows are therefore
// visible only when explicitly requested.
func ListProposals(db *gorm.DB, statusFilter string) ([]StrategyProposal, error) {
	if statusFilter == "" {
		statusFilter = StatusPendingReview
	}
	if !validProposalStatus(statusFilter) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidProposalStatus, statusFilter)
	}

	var proposals []StrategyProposal
	if err := db.
		Where("status = ?", statusFilter).
		Order("created_at DESC, id ASC").
		Find(&proposals).Error; err != nil {
		return nil, fmt.Errorf("ListProposals: query failed: %w", err)
	}
	return proposals, nil
}

// GetProposal returns one proposal or ErrProposalNotFound.
func GetProposal(db *gorm.DB, id uuid.UUID) (*StrategyProposal, error) {
	var p StrategyProposal
	err := db.First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrProposalNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("GetProposal: query failed: %w", err)
	}
	return &p, nil
}

// DecideProposal records a one-way operator decision (accepted or dismissed)
// on a pending_review proposal, stamping decided_at and decided_via. It
// refuses -- with a clear error and no side effects -- an invalid decision,
// an unknown id, a rejected_by_gate proposal, and an already-decided
// proposal. Recording a decision changes only the proposal row itself:
// nothing is applied to any strategy configuration or trading behavior.
func DecideProposal(db *gorm.DB, id uuid.UUID, decision, via string, at time.Time) (*StrategyProposal, error) {
	if !validDecision(decision) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidDecision, decision)
	}

	var decided *StrategyProposal
	err := db.Transaction(func(tx *gorm.DB) error {
		var p StrategyProposal
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrProposalNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("DecideProposal: fetch failed: %w", err)
		}

		if p.Status != StatusPendingReview {
			return fmt.Errorf("%w: proposal %s has status %q", ErrProposalNotDecidable, id, p.Status)
		}

		decidedAt := at
		p.Status = decision
		p.DecidedAt = &decidedAt
		p.DecidedVia = &via
		if err := tx.Save(&p).Error; err != nil {
			return fmt.Errorf("DecideProposal: status update failed: %w", err)
		}

		decided = &p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return decided, nil
}
