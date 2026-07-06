package overfitting

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VerdictStore persists gate verdicts and retrieves the latest verdict for a
// proposal. Implementations: GormVerdictStore (production) and
// FakeVerdictStore (in-memory, tests only), so gate-adjacent orchestration in
// the consuming optimizers is testable without a live database.
type VerdictStore interface {
	Persist(v OverfittingVerdict) error
	FetchLatestByProposal(proposalID uuid.UUID) (*OverfittingVerdict, error)
}

// GormVerdictStore is the production VerdictStore over an already migrated
// *gorm.DB (see MigrateOverfittingCountermeasures).
type GormVerdictStore struct {
	db *gorm.DB
}

// NewGormVerdictStore constructs a GormVerdictStore.
func NewGormVerdictStore(db *gorm.DB) *GormVerdictStore {
	return &GormVerdictStore{db: db}
}

// Persist writes one verdict row. Callers construct the row via
// NewOverfittingVerdict, which pre-assigns the id they reference.
func (s *GormVerdictStore) Persist(v OverfittingVerdict) error {
	if err := s.db.Create(&v).Error; err != nil {
		return fmt.Errorf("GormVerdictStore.Persist: create failed: %w", err)
	}
	return nil
}

// FetchLatestByProposal returns the verdict with the latest computed_at for
// the proposal, or ErrVerdictNotFound when none exists.
func (s *GormVerdictStore) FetchLatestByProposal(proposalID uuid.UUID) (*OverfittingVerdict, error) {
	var v OverfittingVerdict
	err := s.db.
		Where("proposal_id = ?", proposalID).
		Order("computed_at DESC").
		First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrVerdictNotFound, proposalID)
	}
	if err != nil {
		return nil, fmt.Errorf("GormVerdictStore.FetchLatestByProposal: query failed: %w", err)
	}
	return &v, nil
}

// FakeVerdictStore is an in-memory VerdictStore used only by tests. It applies
// the same proposal-kind validation as the production path so a test cannot
// persist a row the database would reject.
type FakeVerdictStore struct {
	verdicts []OverfittingVerdict
}

// NewFakeVerdictStore constructs an empty FakeVerdictStore.
func NewFakeVerdictStore() *FakeVerdictStore {
	return &FakeVerdictStore{}
}

// Persist records the verdict in memory, assigning a fresh id when unset.
func (s *FakeVerdictStore) Persist(v OverfittingVerdict) error {
	if !ProposalKind(v.ProposalKind).Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownProposalKind, v.ProposalKind)
	}
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	s.verdicts = append(s.verdicts, v)
	return nil
}

// FetchLatestByProposal returns the persisted verdict with the latest
// computed_at for the proposal (later-persisted wins a tie), or
// ErrVerdictNotFound when none exists.
func (s *FakeVerdictStore) FetchLatestByProposal(proposalID uuid.UUID) (*OverfittingVerdict, error) {
	var latest *OverfittingVerdict
	for i := range s.verdicts {
		v := &s.verdicts[i]
		if v.ProposalID != proposalID {
			continue
		}
		if latest == nil || !v.ComputedAt.Before(latest.ComputedAt) {
			latest = v
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("%w: %s", ErrVerdictNotFound, proposalID)
	}
	out := *latest
	return &out, nil
}
