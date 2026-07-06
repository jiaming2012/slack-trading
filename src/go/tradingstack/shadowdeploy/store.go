package shadowdeploy

import (
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ShadowStore persists and reads shadow-run evidence. Implementations:
// GormShadowStore (production) and FakeShadowStore (in-memory, tests). The
// store writes ONLY the two shadow tables -- it has no path to any config,
// proposal, scan, or outcome table.
type ShadowStore interface {
	// PersistRun writes one run and its divergence rows atomically. Each
	// divergence's ShadowRunID is set to the run's id.
	PersistRun(run ShadowRun, divergences []ShadowDivergence) error

	// FetchRun returns one run and its divergences (tickers ascending), or
	// ErrRunNotFound.
	FetchRun(id uuid.UUID) (*ShadowRun, []ShadowDivergence, error)

	// ListRuns returns every run, newest first (ties broken by id).
	ListRuns() ([]ShadowRun, error)
}

// GormShadowStore is the production ShadowStore over an already migrated
// *gorm.DB (see MigrateShadowDeployment).
type GormShadowStore struct {
	db *gorm.DB
}

// NewGormShadowStore constructs a GormShadowStore.
func NewGormShadowStore(db *gorm.DB) *GormShadowStore {
	return &GormShadowStore{db: db}
}

// PersistRun writes the run row and its divergence rows in one transaction so
// partial evidence never persists.
func (s *GormShadowStore) PersistRun(run ShadowRun, divergences []ShadowDivergence) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&run).Error; err != nil {
			return fmt.Errorf("GormShadowStore.PersistRun: create run failed: %w", err)
		}
		for i := range divergences {
			divergences[i].ShadowRunID = run.ID
			if err := tx.Create(&divergences[i]).Error; err != nil {
				return fmt.Errorf("GormShadowStore.PersistRun: create divergence failed: %w", err)
			}
		}
		return nil
	})
}

// FetchRun returns one run with its divergences ordered by ticker ascending.
func (s *GormShadowStore) FetchRun(id uuid.UUID) (*ShadowRun, []ShadowDivergence, error) {
	var run ShadowRun
	err := s.db.First(&run, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, fmt.Errorf("%w: %s", ErrRunNotFound, id)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("GormShadowStore.FetchRun: query failed: %w", err)
	}

	var divergences []ShadowDivergence
	if err := s.db.
		Where("shadow_run_id = ?", id).
		Order("ticker ASC, id ASC").
		Find(&divergences).Error; err != nil {
		return nil, nil, fmt.Errorf("GormShadowStore.FetchRun: divergences query failed: %w", err)
	}

	return &run, divergences, nil
}

// ListRuns returns every persisted run, newest first.
func (s *GormShadowStore) ListRuns() ([]ShadowRun, error) {
	var runs []ShadowRun
	if err := s.db.Order("created_at DESC, id ASC").Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("GormShadowStore.ListRuns: query failed: %w", err)
	}
	return runs, nil
}

// FakeShadowStore is an in-memory ShadowStore used by tests. It applies the
// same kind validation as the production path.
type FakeShadowStore struct {
	runs        []ShadowRun
	divergences map[uuid.UUID][]ShadowDivergence

	// PersistCalls counts PersistRun invocations, so guard tests can assert
	// that a refused run persisted nothing.
	PersistCalls int
}

// NewFakeShadowStore constructs an empty FakeShadowStore.
func NewFakeShadowStore() *FakeShadowStore {
	return &FakeShadowStore{divergences: map[uuid.UUID][]ShadowDivergence{}}
}

// PersistRun records the run and divergences in memory, assigning fresh ids
// when unset and validating each divergence's kind.
func (s *FakeShadowStore) PersistRun(run ShadowRun, divergences []ShadowDivergence) error {
	s.PersistCalls++

	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	stored := make([]ShadowDivergence, len(divergences))
	for i, d := range divergences {
		if !validDivergenceKind(d.Kind) {
			return fmt.Errorf("%w: %q", ErrInvalidDivergenceKind, d.Kind)
		}
		if d.ID == uuid.Nil {
			d.ID = uuid.New()
		}
		d.ShadowRunID = run.ID
		stored[i] = d
	}

	s.runs = append(s.runs, run)
	s.divergences[run.ID] = stored
	return nil
}

// FetchRun returns one recorded run and its divergences (tickers ascending),
// or ErrRunNotFound.
func (s *FakeShadowStore) FetchRun(id uuid.UUID) (*ShadowRun, []ShadowDivergence, error) {
	for i := range s.runs {
		if s.runs[i].ID != id {
			continue
		}
		run := s.runs[i]
		divergences := append([]ShadowDivergence(nil), s.divergences[id]...)
		sort.Slice(divergences, func(a, b int) bool {
			if divergences[a].Ticker != divergences[b].Ticker {
				return divergences[a].Ticker < divergences[b].Ticker
			}
			return divergences[a].ID.String() < divergences[b].ID.String()
		})
		return &run, divergences, nil
	}
	return nil, nil, fmt.Errorf("%w: %s", ErrRunNotFound, id)
}

// ListRuns returns the recorded runs, newest first.
func (s *FakeShadowStore) ListRuns() ([]ShadowRun, error) {
	out := append([]ShadowRun(nil), s.runs...)
	sort.Slice(out, func(a, b int) bool {
		if !out[a].CreatedAt.Equal(out[b].CreatedAt) {
			return out[a].CreatedAt.After(out[b].CreatedAt)
		}
		return out[a].ID.String() < out[b].ID.String()
	})
	return out, nil
}
