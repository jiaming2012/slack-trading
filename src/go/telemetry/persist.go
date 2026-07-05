package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MetricRow is one persisted snapshot reading of one series.
type MetricRow struct {
	ID         uint      `gorm:"primaryKey"`
	RecordedAt time.Time `gorm:"index;not null"`
	Name       string    `gorm:"index;not null"`
	Labels     string    `gorm:"type:jsonb;not null;default:'{}'"`
	Kind       string    `gorm:"not null"`
	Value      float64   `gorm:"not null"`
}

func (MetricRow) TableName() string { return "telemetry_metrics" }

// HeartbeatRow is the single durable row per heartbeat source, upserted on
// every beat — durability does not wait for a snapshot tick.
type HeartbeatRow struct {
	ID         uint   `gorm:"primaryKey"`
	SourceKind string `gorm:"not null;uniqueIndex:idx_telemetry_heartbeat_source"`
	Name       string `gorm:"not null;uniqueIndex:idx_telemetry_heartbeat_source"`
	Meta       string `gorm:"type:jsonb;not null;default:'{}'"`
	LastSeenAt time.Time `gorm:"not null"`
	BeatCount  int64     `gorm:"not null;default:0"`
}

func (HeartbeatRow) TableName() string { return "telemetry_heartbeats" }

// AlertRow records one alert through its firing→(acked)→resolved lifecycle.
type AlertRow struct {
	ID             uint      `gorm:"primaryKey"`
	Rule           string    `gorm:"not null;index"`
	Subject        string    `gorm:"not null;index"`
	Message        string    `gorm:"not null"`
	FiredAt        time.Time `gorm:"not null"`
	LastNotifiedAt *time.Time
	AckedAt        *time.Time
	AckedVia       string
	ResolvedAt     *time.Time
}

func (AlertRow) TableName() string { return "telemetry_alerts" }

// Migrate creates the telemetry tables. Idempotent; owns only its own tables
// (mirrors the tradingstack pattern so the module stays extraction-ready).
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&MetricRow{}, &HeartbeatRow{}, &AlertRow{}); err != nil {
		return fmt.Errorf("telemetry.Migrate: auto-migrate failed: %w", err)
	}
	return nil
}

// WriteSnapshot persists the registry's current series as one batch of rows.
func WriteSnapshot(db *gorm.DB, reg *Registry) error {
	points := reg.Snapshot()
	if len(points) == 0 {
		return nil
	}

	rows := make([]MetricRow, 0, len(points))
	for _, p := range points {
		labels, err := json.Marshal(p.Labels)
		if err != nil {
			return fmt.Errorf("WriteSnapshot: marshal labels for %s: %w", p.Name, err)
		}
		rows = append(rows, MetricRow{
			RecordedAt: p.At,
			Name:       p.Name,
			Labels:     string(labels),
			Kind:       string(p.Kind),
			Value:      p.Value,
		})
	}

	if err := db.Create(&rows).Error; err != nil {
		return fmt.Errorf("WriteSnapshot: insert failed: %w", err)
	}
	return nil
}

// UpsertHeartbeat writes the per-source heartbeat row at ingest time.
func UpsertHeartbeat(db *gorm.DB, sourceKind, name string, meta map[string]string, seenAt time.Time) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("UpsertHeartbeat: marshal meta: %w", err)
	}
	if meta == nil {
		metaJSON = []byte("{}")
	}

	row := HeartbeatRow{
		SourceKind: sourceKind,
		Name:       name,
		Meta:       string(metaJSON),
		LastSeenAt: seenAt,
		BeatCount:  1,
	}

	err = db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_kind"}, {Name: "name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"meta":         string(metaJSON),
			"last_seen_at": seenAt,
			"beat_count":   gorm.Expr("telemetry_heartbeats.beat_count + 1"),
		}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("UpsertHeartbeat: upsert failed: %w", err)
	}
	return nil
}

// PruneOnce deletes metric rows older than the retention bound.
func PruneOnce(db *gorm.DB, olderThan time.Time) (int64, error) {
	res := db.Where("recorded_at < ?", olderThan).Delete(&MetricRow{})
	if res.Error != nil {
		return 0, fmt.Errorf("PruneOnce: delete failed: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// StartSnapshotWriter runs the periodic registry flush until ctx is done.
// Failures are logged and retried on the next cycle — persistence problems
// never propagate into trading paths.
func StartSnapshotWriter(ctx context.Context, db *gorm.DB, reg *Registry, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Infof("telemetry: snapshot writer started (interval %s)", interval)
	for {
		select {
		case <-ctx.Done():
			log.Info("telemetry: snapshot writer stopping")
			return
		case <-ticker.C:
			if err := WriteSnapshot(db, reg); err != nil {
				log.Errorf("telemetry: snapshot write failed (will retry next cycle): %v", err)
			}
		}
	}
}

// StartPrune enforces the 30-day retention bound, running once at startup
// and then daily.
func StartPrune(ctx context.Context, db *gorm.DB) {
	run := func() {
		cutoff := time.Now().UTC().Add(-defaultRetention)
		deleted, err := PruneOnce(db, cutoff)
		if err != nil {
			log.Errorf("telemetry: prune failed (will retry next cycle): %v", err)
			return
		}
		if deleted > 0 {
			log.Infof("telemetry: pruned %d metric rows older than %s", deleted, cutoff.Format(time.RFC3339))
		}
	}

	run()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("telemetry: prune stopping")
			return
		case <-ticker.C:
			run()
		}
	}
}
