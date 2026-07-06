package scanneropt

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// ProposalStore persists optimizer proposals and executes the operator
// promote operation. Implementations: GormProposalStore (production) and
// FakeProposalStore (in-memory, tests and the CLI's synthetic mode).
//
// Promote is the ONLY code path in the scanner optimizer that writes to
// scanner_configs, and it is invoked only by the operator promote command.
type ProposalStore interface {
	Persist(p ScannerConfigProposal) error
	List() ([]ScannerConfigProposal, error)
	FetchByID(id uuid.UUID) (*ScannerConfigProposal, error)
	Promote(id uuid.UUID, promotedAt time.Time) (*tradingstack.ScannerConfig, error)
}

// GormProposalStore is the production ProposalStore over an already migrated
// *gorm.DB (see MigrateScannerOptimizer).
type GormProposalStore struct {
	db *gorm.DB
}

// NewGormProposalStore constructs a GormProposalStore.
func NewGormProposalStore(db *gorm.DB) *GormProposalStore {
	return &GormProposalStore{db: db}
}

// Persist writes one proposal row. Callers pre-assign the id (it doubles as
// the gate evidence's ProposalID).
func (s *GormProposalStore) Persist(p ScannerConfigProposal) error {
	if p.VerdictID == uuid.Nil {
		return fmt.Errorf("scanneropt: proposal %s has no verdict id", p.ID)
	}
	if err := s.db.Create(&p).Error; err != nil {
		return fmt.Errorf("GormProposalStore.Persist: create failed: %w", err)
	}
	return nil
}

// List returns every proposal, newest first (ties broken by id for
// deterministic output).
func (s *GormProposalStore) List() ([]ScannerConfigProposal, error) {
	var proposals []ScannerConfigProposal
	if err := s.db.Order("created_at DESC, id ASC").Find(&proposals).Error; err != nil {
		return nil, fmt.Errorf("GormProposalStore.List: query failed: %w", err)
	}
	return proposals, nil
}

// FetchByID returns one proposal or ErrProposalNotFound.
func (s *GormProposalStore) FetchByID(id uuid.UUID) (*ScannerConfigProposal, error) {
	var p ScannerConfigProposal
	err := s.db.First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrProposalNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("GormProposalStore.FetchByID: query failed: %w", err)
	}
	return &p, nil
}

// Promote applies one pending proposal: in a single transaction it inserts a
// new scanner_configs row whose config_json equals the proposal's payload
// verbatim (optimizer_run_id carried over, regime left NULL -- the payload is
// multi-regime) and flips the proposal's status to promoted. It refuses --
// with ErrProposalNotPending and no writes -- any proposal whose status is
// not pending_review, so gate-rejected and already-promoted proposals can
// never be applied. Rolling back a bad promotion uses the existing
// scanner_configs rollback-by-id mechanism from trading-stack-schema.
func (s *GormProposalStore) Promote(id uuid.UUID, promotedAt time.Time) (*tradingstack.ScannerConfig, error) {
	var promoted *tradingstack.ScannerConfig

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var p ScannerConfigProposal
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrProposalNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("GormProposalStore.Promote: fetch failed: %w", err)
		}

		if p.Status != StatusPendingReview {
			return fmt.Errorf("%w: proposal %s has status %q", ErrProposalNotPending, id, p.Status)
		}

		runID := p.OptimizerRunID
		createdAt := promotedAt
		cfg := tradingstack.ScannerConfig{
			CreatedAt:      &createdAt,
			ConfigJSON:     p.ProposedConfigJSON,
			OptimizerRunID: &runID,
		}
		if err := tx.Create(&cfg).Error; err != nil {
			return fmt.Errorf("GormProposalStore.Promote: scanner_configs insert failed: %w", err)
		}

		p.Status = StatusPromoted
		if err := tx.Save(&p).Error; err != nil {
			return fmt.Errorf("GormProposalStore.Promote: status update failed: %w", err)
		}

		promoted = &cfg
		return nil
	})
	if err != nil {
		return nil, err
	}
	return promoted, nil
}

// FakeProposalStore is an in-memory ProposalStore used by tests and the CLI's
// synthetic mode. It applies the same status validation and promote refusal
// rules as the production path.
type FakeProposalStore struct {
	proposals []ScannerConfigProposal
	configs   []tradingstack.ScannerConfig
}

// NewFakeProposalStore constructs an empty FakeProposalStore.
func NewFakeProposalStore() *FakeProposalStore {
	return &FakeProposalStore{}
}

// Persist records the proposal in memory, assigning a fresh id when unset.
func (s *FakeProposalStore) Persist(p ScannerConfigProposal) error {
	if !validProposalStatus(p.Status) {
		return fmt.Errorf("%w: %q", ErrInvalidProposalStatus, p.Status)
	}
	if p.VerdictID == uuid.Nil {
		return fmt.Errorf("scanneropt: proposal %s has no verdict id", p.ID)
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	s.proposals = append(s.proposals, p)
	return nil
}

// List returns the recorded proposals, newest first.
func (s *FakeProposalStore) List() ([]ScannerConfigProposal, error) {
	out := append([]ScannerConfigProposal(nil), s.proposals...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].CreatedAt.After(out[i].CreatedAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// FetchByID returns one recorded proposal or ErrProposalNotFound.
func (s *FakeProposalStore) FetchByID(id uuid.UUID) (*ScannerConfigProposal, error) {
	for i := range s.proposals {
		if s.proposals[i].ID == id {
			out := s.proposals[i]
			return &out, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrProposalNotFound, id)
}

// Promote mirrors the production refusal and write semantics in memory.
func (s *FakeProposalStore) Promote(id uuid.UUID, promotedAt time.Time) (*tradingstack.ScannerConfig, error) {
	for i := range s.proposals {
		if s.proposals[i].ID != id {
			continue
		}
		if s.proposals[i].Status != StatusPendingReview {
			return nil, fmt.Errorf("%w: proposal %s has status %q", ErrProposalNotPending, id, s.proposals[i].Status)
		}

		runID := s.proposals[i].OptimizerRunID
		createdAt := promotedAt
		cfg := tradingstack.ScannerConfig{
			BaseModel:      tradingstack.BaseModel{ID: uuid.New()},
			CreatedAt:      &createdAt,
			ConfigJSON:     append([]byte(nil), s.proposals[i].ProposedConfigJSON...),
			OptimizerRunID: &runID,
		}
		s.configs = append(s.configs, cfg)
		s.proposals[i].Status = StatusPromoted
		out := cfg
		return &out, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrProposalNotFound, id)
}

// Configs returns the scanner_configs rows created by promotions, for test
// assertions.
func (s *FakeProposalStore) Configs() []tradingstack.ScannerConfig {
	return append([]tradingstack.ScannerConfig(nil), s.configs...)
}
