// Command shadow-scan is the operator entry point for shadow config
// deployment (OpenSpec change shadow-config-deployment). Subcommands:
//
//	run              execute one shadow run. --synthetic (default true)
//	                 evaluates the built-in fixture payload pair over the
//	                 built-in observation batch with no database, printing
//	                 the divergence report and outcome comparison. DB mode
//	                 requires the explicit --db flag plus --proposal and an
//	                 optional --from/--to window, and persists the evidence
//	                 to shadow_runs / shadow_divergences.
//	report --run     print a persisted run's configs compared, window,
//	                 divergence report, and coverage-explicit outcome
//	                 comparison.
//
// The command exits non-zero only on an internal error -- never merely
// because divergence was high. Shadow runs are Simulation-only (the mode is
// passed explicitly and anything but simulation is refused), place no
// orders, and never mutate configs or a proposal's status: promotion stays
// exclusively with `task scanner:promote`.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scanneropt"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/shadowdeploy"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <run|report> [flags]", os.Args[0])
	}

	var err error
	switch os.Args[1] {
	case "run":
		err = runCmd(os.Args[2:])
	case "report":
		err = reportCmd(os.Args[2:])
	default:
		log.Fatalf("unknown subcommand %q (want run|report)", os.Args[1])
	}

	if err != nil {
		log.Fatalf("shadow-scan %s: %v", os.Args[1], err)
	}
}

// envOrDefault returns the named environment variable, or def if unset/empty.
// Defaults mirror cmd/scanner-optimizer so this binary works out of the box
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
	synthetic := fs.Bool("synthetic", true, "evaluate the built-in fixture payloads and observations (no database or network required)")
	useDB := fs.Bool("db", false, "run against the database: replay persisted scan_results through both configs and persist the evidence")
	proposalFlag := fs.String("proposal", "", "DB mode: scanner_config_proposals id (uuid) of the shadow candidate (required)")
	fromFlag := fs.String("from", "", "DB mode: scanned_at window start, RFC 3339 (default: 7 days before --to)")
	toFlag := fs.String("to", "", "DB mode: exclusive scanned_at window end, RFC 3339 (default: now)")
	modeFlag := fs.String("mode", string(models.ModeSimulation), "operating mode passed to the Simulation-only guard (anything but simulation is refused)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	mode := models.Mode(*modeFlag)
	if err := mode.Validate(); err != nil {
		return err
	}
	// Guard at the outermost entry point too: even the fixture self-check
	// refuses to present shadow evidence under a real-money mode label.
	if mode != models.ModeSimulation {
		return fmt.Errorf("%w: got mode %q", shadowdeploy.ErrNotSimulation, mode)
	}

	if *useDB {
		return runAgainstDB(mode, *proposalFlag, *fromFlag, *toFlag)
	}
	if !*synthetic {
		return errors.New("nothing to do: pass --db for a database-backed run, or leave --synthetic=true for the self-check")
	}
	return runSynthetic()
}

// runSynthetic evaluates the built-in fixture pair fully in memory: no
// database, no network, nothing persisted.
func runSynthetic() error {
	fixture := shadowdeploy.Synthetic()

	activeDecisions := shadowdeploy.EvaluateConfig(fixture.ActivePayload, fixture.Observations)
	shadowDecisions := shadowdeploy.EvaluateConfig(fixture.ShadowPayload, fixture.Observations)

	report := shadowdeploy.CompareDecisions(activeDecisions, shadowDecisions)
	outcomes := shadowdeploy.CompareOutcomes(activeDecisions, shadowDecisions, fixture.OutcomesByScanResult)

	fmt.Println("shadow-scan: synthetic self-check (built-in fixtures, no database, nothing persisted)")
	fmt.Println()
	fmt.Printf("active payload:  %s\n", fixture.ActivePayload.Version)
	fmt.Printf("shadow payload:  %s\n", fixture.ShadowPayload.Version)
	fmt.Println()
	printReport(report, outcomes)
	return nil
}

// runAgainstDB executes one persisted shadow run: migrations (additive,
// idempotent), guardrails, replay, comparison, persistence.
func runAgainstDB(mode models.Mode, proposalFlag, fromFlag, toFlag string) error {
	if proposalFlag == "" {
		return errors.New("--proposal is required in DB mode")
	}
	proposalID, err := uuid.Parse(proposalFlag)
	if err != nil {
		return fmt.Errorf("invalid --proposal %q: %w", proposalFlag, err)
	}

	now := time.Now().UTC()
	to := now
	if toFlag != "" {
		parsed, err := time.Parse(time.RFC3339, toFlag)
		if err != nil {
			return fmt.Errorf("invalid --to %q: %w", toFlag, err)
		}
		to = parsed
	}
	from := to.Add(-7 * 24 * time.Hour)
	if fromFlag != "" {
		parsed, err := time.Parse(time.RFC3339, fromFlag)
		if err != nil {
			return fmt.Errorf("invalid --from %q: %w", fromFlag, err)
		}
		from = parsed
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
	if err := shadowdeploy.MigrateShadowDeployment(db); err != nil {
		return err
	}

	result, err := shadowdeploy.RunShadow(mode, db, shadowdeploy.NewGormShadowStore(db), proposalID, from, to, now)
	if err != nil {
		return err
	}

	fmt.Printf("shadow run %s persisted\n", result.Run.ID)
	fmt.Printf("proposal:        %s (status %s)\n", result.Proposal.ID, result.Proposal.Status)
	fmt.Printf("active config:   %s\n", activeConfigLabel(result.Run.ActiveConfigID, result.ActivePayload))
	fmt.Printf("shadow payload:  %s\n", result.ShadowPayload.Version)
	fmt.Printf("window:          [%s, %s)\n", result.Run.WindowStart.Format(time.RFC3339), result.Run.WindowEnd.Format(time.RFC3339))
	fmt.Println()
	printReport(result.Report, result.Outcomes)
	fmt.Println()
	fmt.Printf("read it back any time: task scanner:shadow-report -- --run %s\n", result.Run.ID)
	fmt.Println("promotion stays operator-only: task scanner:promote -- --id <proposal-uuid>")
	return nil
}

// activeConfigLabel renders the active baseline: a row id, or the built-in
// default when the scanner_configs table was empty.
func activeConfigLabel(id *uuid.UUID, payload scannercfg.Payload) string {
	if id == nil {
		return fmt.Sprintf("built-in default (%s) -- scanner_configs is empty", payload.Version)
	}
	return fmt.Sprintf("scanner_configs row %s (%s)", id, payload.Version)
}

// printReport prints the divergence report and the coverage-explicit outcome
// comparison, always ending with the structural limitations -- surfaced, not
// hidden.
func printReport(report shadowdeploy.DivergenceReport, outcomes shadowdeploy.OutcomeComparison) {
	fmt.Printf("observations replayed: %d\n", report.TotalObservations)
	fmt.Printf("selected: active=%d shadow=%d both=%d\n", report.SelectedActive, report.SelectedShadow, report.SelectedBoth)
	fmt.Printf("divergence: %.1f%%\n", report.DivergencePct)

	if len(report.ShadowOnly) > 0 {
		fmt.Println("  shadow-only selections:")
		for _, d := range report.ShadowOnly {
			fmt.Printf("    %-6s active_score=%s shadow_score=%s\n", d.Ticker, scoreLabel(d.ActiveScore), scoreLabel(d.ShadowScore))
		}
	}
	if len(report.ActiveOnly) > 0 {
		fmt.Println("  active-only selections:")
		for _, d := range report.ActiveOnly {
			fmt.Printf("    %-6s active_score=%s shadow_score=%s\n", d.Ticker, scoreLabel(d.ActiveScore), scoreLabel(d.ShadowScore))
		}
	}

	fmt.Println()
	fmt.Println("outcome comparison (existing sim_outcomes only):")
	printSide(outcomes.Active)
	printSide(outcomes.Shadow)

	fmt.Println()
	fmt.Println(shadowdeploy.ReplayLimitation)
	fmt.Printf("limitation: %s\n", outcomes.Limitation)
}

func printSide(side shadowdeploy.SideOutcomeSummary) {
	fmt.Printf("  %-6s selections=%d covered=%d (%.1f%%) decided=%d wins=%d losses=%d win_rate=%.3f mean_pnl_pct=%.4f\n",
		side.Side, side.Selections, side.WithOutcomes, side.CoveragePct,
		side.Decided, side.Wins, side.Losses, side.WinRate, side.MeanPnlPct)
}

func scoreLabel(score *float64) string {
	if score == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.4f", *score)
}

func reportCmd(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	runFlag := fs.String("run", "", "shadow run id (uuid) to print (required; omit to list persisted runs)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := openDB()
	if err != nil {
		return err
	}
	store := shadowdeploy.NewGormShadowStore(db)

	if *runFlag == "" {
		return listRuns(store)
	}

	runID, err := uuid.Parse(*runFlag)
	if err != nil {
		return fmt.Errorf("invalid --run %q: %w", *runFlag, err)
	}

	run, divergences, err := store.FetchRun(runID)
	if err != nil {
		return err
	}

	fmt.Printf("shadow run %s (created %s)\n", run.ID, run.CreatedAt.Format(time.RFC3339))
	activeLabel := "built-in default -- scanner_configs was empty at run time"
	if run.ActiveConfigID != nil {
		activeLabel = fmt.Sprintf("scanner_configs row %s", run.ActiveConfigID)
	}
	fmt.Printf("active config:   %s\n", activeLabel)
	fmt.Printf("shadow proposal: %s\n", run.ProposalID)
	fmt.Printf("window:          [%s, %s)\n", run.WindowStart.Format(time.RFC3339), run.WindowEnd.Format(time.RFC3339))
	if run.Synthetic {
		fmt.Println("synthetic:       true (fixture-driven run)")
	}
	fmt.Println()
	fmt.Printf("observations replayed: %d\n", run.TotalObservations)
	fmt.Printf("selected: active=%d shadow=%d both=%d\n", run.SelectedActive, run.SelectedShadow, run.SelectedBoth)
	fmt.Printf("divergence: %.1f%%\n", run.DivergencePct)

	if len(divergences) > 0 {
		fmt.Println("  diverging tickers:")
		for _, d := range divergences {
			fmt.Printf("    %-6s %-11s active_score=%s shadow_score=%s scan_result=%s\n",
				d.Ticker, d.Kind, scoreLabel(d.ActiveScore), scoreLabel(d.ShadowScore), d.ScanResultID)
		}
	}

	fmt.Println()
	var outcomes shadowdeploy.OutcomeComparison
	if err := json.Unmarshal(run.OutcomeSummaryJSON, &outcomes); err != nil {
		fmt.Printf("outcome summary unreadable: %v\n", err)
	} else {
		fmt.Println("outcome comparison (existing sim_outcomes only):")
		printSide(outcomes.Active)
		printSide(outcomes.Shadow)
		fmt.Println()
		fmt.Printf("limitation: %s\n", outcomes.Limitation)
	}
	fmt.Println(shadowdeploy.ReplayLimitation)
	return nil
}

// listRuns prints every persisted shadow run, newest first.
func listRuns(store shadowdeploy.ShadowStore) error {
	runs, err := store.ListRuns()
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		fmt.Println("no shadow runs persisted yet -- run `task scanner:shadow -- --db --proposal <uuid>` first.")
		return nil
	}
	for _, run := range runs {
		fmt.Printf("%s  created=%s  proposal=%s  window=[%s, %s)  observations=%d  divergence=%.1f%%\n",
			run.ID, run.CreatedAt.Format(time.RFC3339), run.ProposalID,
			run.WindowStart.Format(time.RFC3339), run.WindowEnd.Format(time.RFC3339),
			run.TotalObservations, run.DivergencePct)
	}
	return nil
}
