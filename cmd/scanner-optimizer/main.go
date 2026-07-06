// Command scanner-optimizer is the operator entry point for the scanner
// optimizer (OpenSpec change scanner-optimizer). Subcommands:
//
//	run             execute one optimizer cycle. --synthetic (default true)
//	                runs against built-in fixtures with no database and
//	                prints the proposed payload, per-check gate results, and
//	                the would-be proposal status. DB mode requires the
//	                explicit --db flag and persists the verdict and proposal.
//	list            print persisted proposals (id, created_at, status,
//	                verdict pass/fail, evidence summary).
//	promote --id    promote one pending_review proposal into scanner_configs.
//
// The command exits non-zero only on an internal error -- never merely
// because the gate rejected a proposal. Nothing here auto-applies a
// configuration: run persists recommendations; only promote writes to
// scanner_configs, and only when the operator invokes it.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scanneropt"
)

// hotSwapNote reminds the operator that promotion records intent only until
// the scanner learns to consume configs at runtime.
const hotSwapNote = "note: promoted configs take effect only after the future scanner-config-hot-swap change lands -- until then a promoted config is recorded intent, not live scanner behavior."

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <run|list|promote> [flags]", os.Args[0])
	}

	var err error
	switch os.Args[1] {
	case "run":
		err = runCmd(os.Args[2:])
	case "list":
		err = listCmd(os.Args[2:])
	case "promote":
		err = promoteCmd(os.Args[2:])
	default:
		log.Fatalf("unknown subcommand %q (want run|list|promote)", os.Args[1])
	}

	if err != nil {
		log.Fatalf("scanner-optimizer %s: %v", os.Args[1], err)
	}
}

// envOrDefault returns the named environment variable, or def if unset/empty.
// Defaults mirror cmd/tradingstack-db so this binary works out of the box
// against `task db:start`.
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

func runCmd(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	synthetic := fs.Bool("synthetic", true, "run against built-in fixtures (no database or network required)")
	useDB := fs.Bool("db", false, "run against the database: load joined rows, thread them through the validation pipeline, persist verdict and proposal")
	fromFlag := fs.String("from", "", "DB mode: window start, RFC 3339 (default: 90 days before --to)")
	toFlag := fs.String("to", "", "DB mode: exclusive window end, RFC 3339 (default: now)")
	runID := fs.String("run-id", "", "optimizer run id recorded on the proposal (default: generated from the clock)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *useDB {
		return runAgainstDB(*fromFlag, *toFlag, *runID)
	}
	if !*synthetic {
		return errors.New("nothing to do: pass --db for a database-backed run, or leave --synthetic=true for the self-check")
	}
	return runSynthetic()
}

// runSynthetic executes one cycle over the built-in fixtures with in-memory
// stores: no database, no network, nothing persisted.
func runSynthetic() error {
	verdicts := overfitting.NewFakeVerdictStore()
	proposals := scanneropt.NewFakeProposalStore()

	result, err := scanneropt.RunCycle(
		scanneropt.SyntheticWeightedRows(),
		scanneropt.SyntheticBaseline(),
		scanneropt.SyntheticCycleConfig(),
		verdicts,
		proposals,
	)
	if err != nil {
		return err
	}

	fmt.Println("scanner-optimizer: synthetic self-check (built-in fixtures, nothing persisted)")
	fmt.Println()
	printCycle(result, "would-be status")
	return nil
}

// runAgainstDB executes one persisted cycle: migrations (additive,
// idempotent), loader, validation pipeline, tune, gate, persist.
func runAgainstDB(fromFlag, toFlag, runID string) error {
	now := time.Now().UTC()

	to := now
	if toFlag != "" {
		parsed, err := time.Parse(time.RFC3339, toFlag)
		if err != nil {
			return fmt.Errorf("invalid --to %q: %w", toFlag, err)
		}
		to = parsed
	}
	from := to.Add(-90 * 24 * time.Hour)
	if fromFlag != "" {
		parsed, err := time.Parse(time.RFC3339, fromFlag)
		if err != nil {
			return fmt.Errorf("invalid --from %q: %w", fromFlag, err)
		}
		from = parsed
	}
	if runID == "" {
		runID = "scanopt-" + now.Format("20060102T150405Z")
	}

	db, err := openDB()
	if err != nil {
		return err
	}
	if err := overfitting.MigrateOverfittingCountermeasures(db); err != nil {
		return err
	}
	if err := scanneropt.MigrateScannerOptimizer(db); err != nil {
		return err
	}

	input, err := scanneropt.LoadInput(db, from, to)
	if err != nil {
		return err
	}
	fmt.Printf("loaded %d joined training rows in window [%s, %s)\n",
		len(input.Rows), from.Format(time.RFC3339), to.Format(time.RFC3339))

	pipelineResult, err := optvalidation.Run(input, optvalidation.DefaultConfig())
	if err != nil {
		return fmt.Errorf("validation pipeline failed: %w", err)
	}
	fmt.Printf("validation pipeline: kept=%d timestamp_violations=%d regime_dropped=%d fidelity_dropped=%d drifted_features=%v\n",
		len(pipelineResult.Clean),
		len(pipelineResult.Violations),
		len(pipelineResult.DroppedByRegime),
		len(pipelineResult.DroppedByFidelity),
		pipelineResult.DistributionReport.Drifted,
	)

	baseline, err := scanneropt.LoadActiveBaseline(db)
	if err != nil {
		return err
	}
	fmt.Printf("active baseline payload version: %s\n\n", baseline.Version)

	result, err := scanneropt.RunCycle(
		pipelineResult.Clean,
		baseline,
		scanneropt.CycleConfig{
			Now:        now,
			RunID:      runID,
			Version:    now.Format(time.RFC3339),
			GateConfig: overfitting.DefaultConfig(),
		},
		overfitting.NewGormVerdictStore(db),
		scanneropt.NewGormProposalStore(db),
	)
	if err != nil {
		return err
	}

	printCycle(result, "persisted status")
	fmt.Println(hotSwapNote)
	return nil
}

// printCycle prints the proposed payload, per-check gate results, and the
// proposal status.
func printCycle(result *scanneropt.CycleResult, statusLabel string) {
	payloadJSON, err := result.Payload.Marshal()
	if err != nil {
		payloadJSON = []byte(fmt.Sprintf("<marshal error: %v>", err))
	}
	fmt.Printf("proposed payload:\n%s\n\n", payloadJSON)

	fmt.Printf("gate verdict: %s (proposal %s)\n", passFail(result.Verdict.Passed), result.Proposal.ID)
	for _, c := range result.Verdict.Checks {
		fmt.Printf("  [%s] %s\n", passFail(c.Passed), c.Name)
		fmt.Printf("    observed:  %s\n", formatValues(c.Observed))
		fmt.Printf("    threshold: %s\n", formatValues(c.Threshold))
		fmt.Printf("    detail:    %s\n", c.Detail)
	}
	fmt.Println()
	fmt.Printf("%s: %s\n", statusLabel, result.Proposal.Status)
}

func listCmd(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := openDB()
	if err != nil {
		return err
	}

	proposals, err := scanneropt.NewGormProposalStore(db).List()
	if err != nil {
		return err
	}

	if len(proposals) == 0 {
		fmt.Println("no scanner config proposals persisted yet -- run `task scanner:optimize -- --db` first.")
		return nil
	}

	for _, p := range proposals {
		verdictLabel := "unknown"
		var verdictRow overfitting.OverfittingVerdict
		if err := db.First(&verdictRow, "id = ?", p.VerdictID).Error; err == nil {
			verdictLabel = passFail(verdictRow.Passed)
		}

		fmt.Printf("%s  created=%s  status=%s  verdict=%s  run=%s\n",
			p.ID, p.CreatedAt.Format(time.RFC3339), p.Status, verdictLabel, p.OptimizerRunID)

		if evidence, err := scanneropt.UnmarshalEvidence(p.EvidenceJSON); err == nil {
			fmt.Printf("    evidence: samples=%d trials=%d folds=%d in_sample_mean=%.6f oos_mean=%.6f oos_n=%d\n",
				evidence.SampleSize, evidence.TrialsCount, len(evidence.Folds),
				evidence.InSample.Mean, evidence.OutOfSample.Mean, evidence.OutOfSample.SampleSize)
		}
	}

	fmt.Println()
	fmt.Println(hotSwapNote)
	return nil
}

func promoteCmd(args []string) error {
	fs := flag.NewFlagSet("promote", flag.ExitOnError)
	idFlag := fs.String("id", "", "proposal id (uuid) to promote (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *idFlag == "" {
		return errors.New("--id is required")
	}
	id, err := uuid.Parse(*idFlag)
	if err != nil {
		return fmt.Errorf("invalid --id %q: %w", *idFlag, err)
	}

	db, err := openDB()
	if err != nil {
		return err
	}

	cfg, err := scanneropt.NewGormProposalStore(db).Promote(id, time.Now().UTC())
	if err != nil {
		return err
	}

	fmt.Printf("promoted proposal %s -> scanner_configs row %s\n", id, cfg.ID)
	fmt.Println(hotSwapNote)
	return nil
}

func passFail(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

// formatValues renders a value map with sorted keys so output is
// deterministic run-to-run.
func formatValues(m map[string]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := ""
	for i, k := range keys {
		if i > 0 {
			out += "  "
		}
		out += fmt.Sprintf("%s=%.4f", k, m[k])
	}
	return out
}
