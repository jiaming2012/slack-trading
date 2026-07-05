package retention

import "errors"

// Sentinel errors for the retention package's input validation.
var (
	// ErrNilDB is returned when a retention function is called with a nil
	// *gorm.DB.
	ErrNilDB = errors.New("retention: db is nil")

	// ErrEmptyTableName is returned when tableName is empty.
	ErrEmptyTableName = errors.New("retention: tableName is empty")

	// ErrInvalidIdentifier is returned when tableName or partitionName
	// contains characters outside [a-zA-Z0-9_], since both are interpolated
	// into DDL as unquoted identifiers.
	ErrInvalidIdentifier = errors.New("retention: identifier must match [a-zA-Z0-9_]+")

	// ErrNilArchiver is returned when RunLive is called with a nil
	// ColdStorageArchiver.
	ErrNilArchiver = errors.New("retention: archiver is nil")
)
