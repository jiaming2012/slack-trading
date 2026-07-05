package partitioning

import "errors"

// Sentinel errors for the partitioning package's input validation.
var (
	// ErrNilDB is returned when EnsureMonthlyPartitions is called with a nil
	// *gorm.DB.
	ErrNilDB = errors.New("partitioning: db is nil")

	// ErrEmptyTableName is returned when tableName is empty.
	ErrEmptyTableName = errors.New("partitioning: tableName is empty")

	// ErrInvalidIdentifier is returned when tableName or partitionKeyCol
	// contains characters outside [a-zA-Z0-9_], since both are interpolated
	// into DDL as unquoted identifiers.
	ErrInvalidIdentifier = errors.New("partitioning: identifier must match [a-zA-Z0-9_]+")

	// ErrNegativeHorizon is returned when horizonMonths is negative.
	ErrNegativeHorizon = errors.New("partitioning: horizonMonths must be >= 0")

	// ErrPartitionKeyMismatch is returned when tableName is already a
	// partitioned table but its actual partition key column does not match
	// the partitionKeyCol argument.
	ErrPartitionKeyMismatch = errors.New("partitioning: table is partitioned on a different column than partitionKeyCol")
)
