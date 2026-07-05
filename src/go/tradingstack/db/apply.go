// Package tradingstackdb embeds and applies the raw-SQL partitioning
// migration for the v4 trading-stack tables. It exists so the exact same SQL
// file that infra/migrate.py registers and applies in production (via psql)
// is also the one Go tests apply against a testcontainers Postgres -- one
// migration file, two callers, zero drift.
package tradingstackdb

import (
	_ "embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed partitioning.sql
var partitioningSQL string

// ApplyPartitioningMigration executes partitioning.sql against db, statement
// by statement. It is idempotent -- see partitioning.sql's header comment.
func ApplyPartitioningMigration(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ApplyPartitioningMigration: db is nil")
	}

	for _, stmt := range splitStatements(partitioningSQL) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("ApplyPartitioningMigration: statement failed: %w\nstatement:\n%s", err, stmt)
		}
	}

	return nil
}

// splitStatements splits sql into individually-executable statements,
// respecting $$ dollar-quoted blocks (e.g. `DO $$ ... $$;`) so a semicolon
// inside a PL/pgSQL block does not end the statement early. Mirrors the
// Python splitter in infra/migrate.py so both apply identical statement
// boundaries against the identical file.
func splitStatements(sql string) []string {
	var statements []string
	var current []string
	inDollar := false

	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") && len(current) == 0 {
			continue
		}

		current = append(current, line)

		if strings.Count(line, "$$")%2 == 1 {
			inDollar = !inDollar
		}

		if !inDollar && strings.Contains(line, ";") {
			stmt := strings.TrimSpace(strings.Join(current, "\n"))
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current = nil
		}
	}

	if len(current) > 0 {
		stmt := strings.TrimSpace(strings.Join(current, "\n"))
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}
