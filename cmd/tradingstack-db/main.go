// Command tradingstack-db is the operator entry point for the v4
// trading-stack partitioning and retention jobs. It is invoked via the
// Taskfile targets db:partition:ensure, db:retention:dry-run, and
// db:retention:run -- see OpenSpec change db-partitioning-retention.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/partitioning"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/retention"
)

// envOrDefault returns the named environment variable, or def if unset/empty.
// Defaults mirror infra/migrate.py's local docker-compose defaults so this
// binary works out of the box against `task db:start`.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func openDB() (*gorm.DB, error) {
	host := envOrDefault("POSTGRES_HOST", "localhost")
	port := envOrDefault("POSTGRES_PORT", "5432")
	user := envOrDefault("POSTGRES_USER", "grodt")
	password := envOrDefault("POSTGRES_PASSWORD", "test747")
	dbName := envOrDefault("POSTGRES_DB", "playground")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC", host, user, password, dbName, port)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("openDB: failed to connect: %w", err)
	}
	return db, nil
}

// partitionedTables mirrors retention.PartitionedTables' table names, paired
// with the partition-key column each is keyed on, needed by
// EnsureMonthlyPartitions but not carried by retention.PartitionedTables.
var partitionedTables = []struct {
	Table string
	Col   string
}{
	{"scan_results", "scanned_at"},
	{"sim_outcomes", "simulated_at"},
}

// defaultHorizonMonths matches design.md Decision 4: current month + 3 ahead.
const defaultHorizonMonths = 3

func runPartitionEnsure(ctx context.Context, db *gorm.DB) error {
	horizon := defaultHorizonMonths
	if v := os.Getenv("TRADINGSTACK_PARTITION_HORIZON_MONTHS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("runPartitionEnsure: invalid TRADINGSTACK_PARTITION_HORIZON_MONTHS=%q: %w", v, err)
		}
		horizon = parsed
	}

	for _, t := range partitionedTables {
		present, err := partitioning.EnsureMonthlyPartitions(ctx, db, t.Table, t.Col, horizon)
		if err != nil {
			return fmt.Errorf("runPartitionEnsure: %s: %w", t.Table, err)
		}
		fmt.Printf("%s: %d partition(s) present (current month + %d month horizon):\n", t.Table, len(present), horizon)
		for _, name := range present {
			fmt.Printf("  - %s\n", name)
		}
	}
	return nil
}

func runRetentionDryRun(ctx context.Context, db *gorm.DB) error {
	reports, err := retention.RunDryRun(ctx, db, retention.DefaultRetentionWindow)
	if err != nil {
		return fmt.Errorf("runRetentionDryRun: %w", err)
	}
	for _, r := range reports {
		fmt.Printf("%s: %d partition(s) eligible for archival\n", r.TableName, len(r.Eligible))
		for _, p := range r.Eligible {
			fmt.Printf("  - %s (range %s to %s)\n", p.Name, p.RangeStart.Format("2006-01-02"), p.RangeEnd.Format("2006-01-02"))
		}
	}
	return nil
}

func runRetentionRun(ctx context.Context, db *gorm.DB) error {
	reports, err := retention.RunLive(ctx, db, retention.StubArchiver{}, retention.DefaultRetentionWindow)
	if err != nil {
		return fmt.Errorf("runRetentionRun: %w", err)
	}
	for _, r := range reports {
		fmt.Printf("%s: %d partition(s) detached and archived (stub, no data deleted)\n", r.TableName, len(r.Detached))
		for _, name := range r.Detached {
			fmt.Printf("  - %s\n", name)
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <partition-ensure|retention-dry-run|retention-run>", os.Args[0])
	}

	ctx := context.Background()
	db, err := openDB()
	if err != nil {
		log.Fatalf("failed to open database connection: %v", err)
	}

	switch os.Args[1] {
	case "partition-ensure":
		err = runPartitionEnsure(ctx, db)
	case "retention-dry-run":
		err = runRetentionDryRun(ctx, db)
	case "retention-run":
		err = runRetentionRun(ctx, db)
	default:
		log.Fatalf("unknown subcommand %q (want partition-ensure|retention-dry-run|retention-run)", os.Args[1])
	}

	if err != nil {
		log.Fatalf("%v", err)
	}
}
