package crowding

import (
	"fmt"

	"gorm.io/gorm"
)

// CrowdingStore persists a computed CrowdingMetric and its associated flagged
// candidates. Implementations: GormCrowdingStore (production) and
// FakeCrowdingStore (in-memory, tests only).
type CrowdingStore interface {
	Persist(metric CrowdingMetric, flagged []CrowdingFlaggedCandidate) error
}

// GormCrowdingStore is the production CrowdingStore, writing the metric row
// and its flagged-candidate rows in a single transaction.
type GormCrowdingStore struct {
	db *gorm.DB
}

// NewGormCrowdingStore constructs a GormCrowdingStore over an already
// migrated *gorm.DB (see MigrateCrowdingDetection).
func NewGormCrowdingStore(db *gorm.DB) *GormCrowdingStore {
	return &GormCrowdingStore{db: db}
}

// Persist writes metric, then writes each flagged candidate with its
// CrowdingMetricID set to the persisted metric's id, all inside one
// transaction so partial writes never occur.
func (s *GormCrowdingStore) Persist(metric CrowdingMetric, flagged []CrowdingFlaggedCandidate) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&metric).Error; err != nil {
			return fmt.Errorf("GormCrowdingStore.Persist: create metric failed: %w", err)
		}

		for i := range flagged {
			flagged[i].CrowdingMetricID = metric.ID
			if err := tx.Create(&flagged[i]).Error; err != nil {
				return fmt.Errorf("GormCrowdingStore.Persist: create flagged candidate failed: %w", err)
			}
		}

		return nil
	})
}

// FakeCrowdingStore is an in-memory CrowdingStore used only by tests. It
// records the last-persisted metric and flagged candidates verbatim, with no
// database dependency.
type FakeCrowdingStore struct {
	LastMetric   CrowdingMetric
	LastFlagged  []CrowdingFlaggedCandidate
	PersistCalls int
}

// NewFakeCrowdingStore constructs an empty FakeCrowdingStore.
func NewFakeCrowdingStore() *FakeCrowdingStore {
	return &FakeCrowdingStore{}
}

// Persist records metric and flagged verbatim for later assertion.
func (s *FakeCrowdingStore) Persist(metric CrowdingMetric, flagged []CrowdingFlaggedCandidate) error {
	s.LastMetric = metric
	s.LastFlagged = flagged
	s.PersistCalls++
	return nil
}
