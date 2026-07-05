// Package partitioning provisions native PostgreSQL RANGE partitions ahead of
// need for the v4 trading-stack tables (scan_results, sim_outcomes). It never
// creates a DEFAULT partition: an insert into an unprovisioned month is meant
// to fail loudly rather than be silently absorbed. See
// openspec/changes/db-partitioning-retention/design.md, Decisions 4 and 5.
package partitioning

import (
	"context"
	"fmt"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// identifierRe matches the safe subset of characters this package allows for
// table/column names before they are interpolated into DDL as unquoted
// identifiers (Postgres has no way to bind-parameter an identifier).
var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// EnsureMonthlyPartitions creates any missing monthly RANGE partitions for
// tableName, covering the current calendar month through horizonMonths ahead
// (inclusive), for a table already PARTITION BY RANGE (partitionKeyCol).
//
// It is idempotent: partitions that already exist are left untouched
// (CREATE TABLE IF NOT EXISTS), and re-running with no elapsed time creates
// zero new partitions and does not error. It never creates a DEFAULT
// partition.
func EnsureMonthlyPartitions(ctx context.Context, db *gorm.DB, tableName, partitionKeyCol string, horizonMonths int) ([]string, error) {
	if db == nil {
		return nil, ErrNilDB
	}
	if tableName == "" {
		return nil, ErrEmptyTableName
	}
	if !identifierRe.MatchString(tableName) {
		return nil, fmt.Errorf("%w: tableName=%q", ErrInvalidIdentifier, tableName)
	}
	if !identifierRe.MatchString(partitionKeyCol) {
		return nil, fmt.Errorf("%w: partitionKeyCol=%q", ErrInvalidIdentifier, partitionKeyCol)
	}
	if horizonMonths < 0 {
		return nil, ErrNegativeHorizon
	}

	if err := verifyPartitionKey(ctx, db, tableName, partitionKeyCol); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	created := make([]string, 0, horizonMonths+1)
	for i := 0; i <= horizonMonths; i++ {
		start := monthStart.AddDate(0, i, 0)
		end := start.AddDate(0, 1, 0)
		partName := partitionName(tableName, start)

		stmt := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
			partName, tableName, start.Format("2006-01-02"), end.Format("2006-01-02"),
		)
		if err := db.WithContext(ctx).Exec(stmt).Error; err != nil {
			return created, fmt.Errorf("EnsureMonthlyPartitions: failed to create partition %s of %s: %w", partName, tableName, err)
		}

		log.Debugf("partitioning: ensured %s exists (partition of %s for %s)", partName, tableName, start.Format("2006-01"))
		created = append(created, partName)
	}

	return created, nil
}

// partitionName builds the deterministic partition table name for a given
// calendar month, e.g. scan_results_y2026_m07.
func partitionName(tableName string, month time.Time) string {
	return fmt.Sprintf("%s_y%04d_m%02d", tableName, month.Year(), int(month.Month()))
}

// verifyPartitionKey confirms tableName is a partitioned table whose sole
// partition key column matches partitionKeyCol, when tableName already
// exists as a partitioned relation. If tableName does not yet exist as a
// partitioned relation (e.g. it hasn't been migrated yet), verification is
// skipped -- the subsequent CREATE TABLE ... PARTITION OF call will fail with
// a clear Postgres error instead.
func verifyPartitionKey(ctx context.Context, db *gorm.DB, tableName, partitionKeyCol string) error {
	var actualCols []string
	err := db.WithContext(ctx).Raw(`
		SELECT a.attname
		FROM pg_partitioned_table pt
		JOIN pg_class c ON c.oid = pt.partrelid
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(pt.partattrs)
		WHERE c.relname = ?
	`, tableName).Scan(&actualCols).Error
	if err != nil {
		return fmt.Errorf("verifyPartitionKey: query failed for %s: %w", tableName, err)
	}

	if len(actualCols) == 0 {
		// Not (yet) a partitioned relation -- nothing to verify.
		return nil
	}

	for _, c := range actualCols {
		if c == partitionKeyCol {
			return nil
		}
	}

	return fmt.Errorf("%w: table=%s expected=%s actual=%v", ErrPartitionKeyMismatch, tableName, partitionKeyCol, actualCols)
}
