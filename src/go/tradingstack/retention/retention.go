// Package retention implements the 12-month archival policy for the v4
// trading-stack partitioned tables (scan_results, sim_outcomes): it finds
// partitions whose entire date range has aged past the retention window,
// detaches them from their parent table (data-preserving -- see
// DetachPartition), and hands each detached partition to a swappable
// ColdStorageArchiver hook. feature_distributions and strategy_ev_weights are
// never partitioned and are never passed through this package.
package retention

import (
	"context"
	"fmt"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// PartitionedTables lists the only tables this package ever operates on.
// feature_distributions, strategy_ev_weights, simulator_fidelity, and
// scanner_configs are deliberately excluded -- see design.md Non-Goals.
var PartitionedTables = []struct {
	TableName string
}{
	{TableName: "scan_results"},
	{TableName: "sim_outcomes"},
}

// identifierRe matches the safe subset of characters this package allows for
// table/partition names before they are interpolated into DDL as unquoted
// identifiers.
var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// partitionBoundRe extracts the FROM/TO timestamp literals out of Postgres's
// `pg_get_expr(relpartbound, oid)` rendering, e.g.:
//
//	FOR VALUES FROM ('2026-07-01 00:00:00+00') TO ('2026-08-01 00:00:00+00')
var partitionBoundRe = regexp.MustCompile(`FROM \('([^']+)'\) TO \('([^']+)'\)`)

// ColdStorageArchiver is invoked exactly once per detached partition. The
// shipped implementation (StubArchiver) records intent only -- see its
// doc comment. A real cold-storage backend is out of scope for this change.
type ColdStorageArchiver interface {
	Archive(ctx context.Context, partitionName string) error
}

// StubArchiver is the shipped ColdStorageArchiver implementation. It never
// makes a network call, never uploads data anywhere, and never drops or
// deletes the detached partition table -- it only records archival intent
// via a log line. Wiring a real cold-storage backend (S3/GCS/etc.) is a
// distinct, future change.
type StubArchiver struct{}

// Archive logs the partition name as archived (in intent only) and returns
// nil. It performs no I/O beyond the log line.
func (StubArchiver) Archive(_ context.Context, partitionName string) error {
	log.Infof("retention: stub archiver recording archive intent for partition %s (no upload performed, data left in place)", partitionName)
	return nil
}

// PartitionInfo describes one partition found attached to a parent table.
type PartitionInfo struct {
	Name       string
	RangeStart time.Time
	RangeEnd   time.Time
}

// FindEligiblePartitions returns every partition of tableName whose entire
// date range ends at or before now - retentionWindow. Partitions whose range
// extends into the retention window are excluded. Only partitions currently
// attached to tableName are considered -- a partition already detached (by
// a prior run) is no longer inherited by tableName and so is never
// rediscovered, which is what makes repeated live runs idempotent.
func FindEligiblePartitions(ctx context.Context, db *gorm.DB, tableName string, retentionWindow time.Duration) ([]PartitionInfo, error) {
	if db == nil {
		return nil, ErrNilDB
	}
	if tableName == "" {
		return nil, ErrEmptyTableName
	}
	if !identifierRe.MatchString(tableName) {
		return nil, fmt.Errorf("%w: tableName=%q", ErrInvalidIdentifier, tableName)
	}

	type row struct {
		Name  string
		Bound string
	}
	var rows []row
	err := db.WithContext(ctx).Raw(`
		SELECT c.relname AS name, pg_get_expr(c.relpartbound, c.oid) AS bound
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = ?::regclass
		ORDER BY c.relname
	`, tableName).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("FindEligiblePartitions: query failed for %s: %w", tableName, err)
	}

	cutoff := time.Now().UTC().Add(-retentionWindow)

	eligible := make([]PartitionInfo, 0, len(rows))
	for _, r := range rows {
		m := partitionBoundRe.FindStringSubmatch(r.Bound)
		if m == nil {
			// DEFAULT partition or unrecognized bound shape -- this package
			// never creates a DEFAULT partition, but skip defensively rather
			// than mis-parse.
			continue
		}

		start, err := time.Parse("2006-01-02 15:04:05-07", m[1])
		if err != nil {
			return nil, fmt.Errorf("FindEligiblePartitions: failed to parse range start %q for %s: %w", m[1], r.Name, err)
		}
		end, err := time.Parse("2006-01-02 15:04:05-07", m[2])
		if err != nil {
			return nil, fmt.Errorf("FindEligiblePartitions: failed to parse range end %q for %s: %w", m[2], r.Name, err)
		}

		if !end.After(cutoff) {
			eligible = append(eligible, PartitionInfo{Name: r.Name, RangeStart: start, RangeEnd: end})
		}
	}

	return eligible, nil
}

// DetachPartition issues ALTER TABLE <tableName> DETACH PARTITION
// <partitionName>. Detaching never deletes or modifies the partition's data:
// it only removes the partition from the parent's partition set, so the
// parent no longer scans or returns it, while the detached table remains
// queryable directly by name with all of its original rows intact.
func DetachPartition(ctx context.Context, db *gorm.DB, tableName, partitionName string) error {
	if db == nil {
		return ErrNilDB
	}
	if !identifierRe.MatchString(tableName) {
		return fmt.Errorf("%w: tableName=%q", ErrInvalidIdentifier, tableName)
	}
	if !identifierRe.MatchString(partitionName) {
		return fmt.Errorf("%w: partitionName=%q", ErrInvalidIdentifier, partitionName)
	}

	stmt := fmt.Sprintf(`ALTER TABLE %s DETACH PARTITION %s`, tableName, partitionName)
	if err := db.WithContext(ctx).Exec(stmt).Error; err != nil {
		return fmt.Errorf("DetachPartition: failed to detach %s from %s: %w", partitionName, tableName, err)
	}

	log.Infof("retention: detached partition %s from %s (data preserved, table remains queryable directly)", partitionName, tableName)
	return nil
}

// TableReport summarizes one table's retention pass.
type TableReport struct {
	TableName string
	Eligible  []PartitionInfo
	Detached  []string
}

// DefaultRetentionWindow is the default 12-month archival threshold named in
// the proposal and design.
const DefaultRetentionWindow = 12 * 30 * 24 * time.Hour

// RunDryRun reports, for every table in PartitionedTables, which partitions
// are eligible for archival without detaching or archiving any of them.
func RunDryRun(ctx context.Context, db *gorm.DB, retentionWindow time.Duration) ([]TableReport, error) {
	reports := make([]TableReport, 0, len(PartitionedTables))
	for _, t := range PartitionedTables {
		eligible, err := FindEligiblePartitions(ctx, db, t.TableName, retentionWindow)
		if err != nil {
			return reports, err
		}
		reports = append(reports, TableReport{TableName: t.TableName, Eligible: eligible})
	}
	return reports, nil
}

// RunLive detaches every eligible partition (for every table in
// PartitionedTables) and invokes archiver.Archive exactly once per detached
// partition. Re-running immediately afterward finds zero newly-eligible
// partitions (already-detached partitions are no longer inherited by their
// former parent) and completes without error or duplicate archiver
// invocation.
func RunLive(ctx context.Context, db *gorm.DB, archiver ColdStorageArchiver, retentionWindow time.Duration) ([]TableReport, error) {
	if archiver == nil {
		return nil, ErrNilArchiver
	}

	reports := make([]TableReport, 0, len(PartitionedTables))
	for _, t := range PartitionedTables {
		eligible, err := FindEligiblePartitions(ctx, db, t.TableName, retentionWindow)
		if err != nil {
			return reports, err
		}

		report := TableReport{TableName: t.TableName, Eligible: eligible}
		for _, p := range eligible {
			if err := DetachPartition(ctx, db, t.TableName, p.Name); err != nil {
				return reports, err
			}
			if err := archiver.Archive(ctx, p.Name); err != nil {
				return reports, fmt.Errorf("RunLive: archiver failed for partition %s: %w", p.Name, err)
			}
			report.Detached = append(report.Detached, p.Name)
		}
		reports = append(reports, report)
	}

	return reports, nil
}
