package data

// Persistence for deferred option auto-closes (wire-companion-stops,
// adversarial-review finding 2): deferrals originate from drain-once
// assignment/expiration events, so they are written on deferral, deleted on
// successful placement, and reloaded at playground load — a halt followed by a
// process restart must not silently drop a close.

import (
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

// SaveDeferredAutoClose persists one deferral and stamps its record ID back
// onto the in-memory deferral so a later successful placement can delete it.
func (s *DatabaseService) SaveDeferredAutoClose(playgroundID uuid.UUID, d *backtester_models.DeferredAutoClose) error {
	rec, err := d.ToRecord(playgroundID)
	if err != nil {
		return fmt.Errorf("SaveDeferredAutoClose: %w", err)
	}

	if err := s.db.Create(rec).Error; err != nil {
		return fmt.Errorf("SaveDeferredAutoClose: failed to persist deferred auto-close for order %d: %w", d.SourceOrderID, err)
	}

	d.RecordID = rec.ID
	return nil
}

// DeleteDeferredAutoClose hard-deletes a persisted deferral (Unscoped: a
// soft-deleted row must never resurrect a committed close on restart). A zero
// record ID — a deferral that was never persisted — is a no-op.
func (s *DatabaseService) DeleteDeferredAutoClose(recordID uint) error {
	if recordID == 0 {
		return nil
	}

	if err := s.db.Unscoped().Delete(&backtester_models.DeferredAutoCloseRecord{}, recordID).Error; err != nil {
		return fmt.Errorf("DeleteDeferredAutoClose: failed to delete deferred auto-close row %d: %w", recordID, err)
	}

	return nil
}

// LoadDeferredAutoCloses returns every persisted deferral for the playground
// in insertion order.
func (s *DatabaseService) LoadDeferredAutoCloses(playgroundID uuid.UUID) ([]*backtester_models.DeferredAutoClose, error) {
	var recs []backtester_models.DeferredAutoCloseRecord
	if err := s.db.Where("playground_id = ?", playgroundID).Order("id asc").Find(&recs).Error; err != nil {
		return nil, fmt.Errorf("LoadDeferredAutoCloses: failed to load deferred auto-closes for playground %s: %w", playgroundID, err)
	}

	out := make([]*backtester_models.DeferredAutoClose, 0, len(recs))
	for i := range recs {
		d, err := recs[i].ToDeferredAutoClose()
		if err != nil {
			return nil, fmt.Errorf("LoadDeferredAutoCloses: %w", err)
		}
		out = append(out, d)
	}

	return out, nil
}

// restoreDeferredAutoCloses rehydrates a freshly loaded playground's deferred
// auto-closes from the database (the restart path). Stale rows — deferrals
// whose source order no longer has remaining open quantity — are deleted so
// they can never replay a committed close.
func (s *DatabaseService) restoreDeferredAutoCloses(p *backtester_models.Playground) error {
	deferrals, err := s.LoadDeferredAutoCloses(p.ID)
	if err != nil {
		return err
	}

	if len(deferrals) == 0 {
		return nil
	}

	stale := p.RestoreDeferredAutoCloses(deferrals)
	for _, d := range stale {
		if err := s.DeleteDeferredAutoClose(d.RecordID); err != nil {
			log.Warnf("restoreDeferredAutoCloses: failed to delete stale deferred auto-close row %d: %v", d.RecordID, err)
		}
	}

	return nil
}
