// Command strategy-optimizer is the operator entry point for the strategy
// optimizer (OpenSpec change strategy-optimizer). Subcommands:
//
//	generate        execute one generation run. Default (no --db) runs the
//	                built-in synthetic fixtures in dry-run mode: no database,
//	                nothing persisted. --db loads scan_results/sim_outcomes
//	                plus the pipeline reference tables, threads them through
//	                the validation pipeline, generates and gates candidates,
//	                and persists proposals and verdicts (unless --dry-run).
//	list            print proposals, defaulting to the pending_review review
//	                queue; --status selects another status (rejected_by_gate
//	                rows are visible ONLY via that explicit filter).
//	show --id       inspect one proposal: rationale, evidence, and its
//	                overfitting-gate verdict with per-check results.
//	decide --id --decision accepted|dismissed
//	                record a one-way operator decision on a pending_review
//	                proposal (stamps decided_via=cli).
//
// The command exits non-zero only on an internal error: generating zero
// proposals, and having every candidate rejected by the gate, are normal
// reportable outcomes. Every proposal is a recommendation requiring operator
// action -- NOTHING here applies a proposal to any strategy configuration or
// trading behavior.
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
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/stratopt"
)

// recommendationNote labels every proposal listing so operators never mistake
// optimizer output for applied configuration.
const recommendationNote = "recommendation -- requires operator action; nothing is ever auto-applied to any trading config."

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <generate|list|show|decide> [flags]", os.Args[0])
	}

	var err error
	switch os.Args[1] {
	case "generate":
		err = generateCmd(os.Args[2:])
	case "list":
		err = listCmd(os.Args[2:])
	case "show":
		err = showCmd(os.Args[2:])
	case "decide":
		err = decideCmd(os.Args[2:])
	default:
		log.Fatalf("unknown subcommand %q (want generate|list|show|decide)", os.Args[1])
	}

	if err != nil {
		log.Fatalf("strategy-optimizer %s: %v", os.Args[1], err)
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

// migrate runs the migrations this command depends on, in their documented
// order: the overfitting gate's verdict table first (the FK parent), then the
// strategy optimizer's proposal table. Both are additive and idempotent.
func migrate(db *gorm.DB) error {
	if err := overfitting.MigrateOverfittingCountermeasures(db); err != nil {
		return err
	}
	return stratopt.MigrateStrategyOptimizer(db)
}

func generateCmd(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	synthetic := fs.Bool("synthetic", true, "run against built-in fixtures (no database or network required); implied unless --db is set")
	useDB := fs.Bool("db", false, "run against the database: load joined rows, thread them through the validation pipeline, generate, gate, and persist")
	dryRun := fs.Bool("dry-run", false, "DB mode: print without persisting -- the gate runs against an in-memory verdict store and no proposal row is written")
	fromFlag := fs.String("from", "", "DB mode: window start, RFC 3339 (default: 90 days before --to)")
	toFlag := fs.String("to", "", "DB mode: exclusive window end, RFC 3339 (default: now)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *useDB {
		return generateAgainstDB(*fromFlag, *toFlag, *dryRun)
	}
	if !*synthetic {
		return errors.New("nothing to do: pass --db for a database-backed run, or leave --synthetic=true for the self-check")
	}
	return generateSynthetic()
}

// generateSynthetic executes one run over the built-in fixtures with an
// in-memory verdict store: no database, no network, nothing persisted.
func generateSynthetic() error {
	input, outcomes := stratopt.SyntheticDataset()

	result, err := stratopt.GenerateRun(input, outcomes, stratopt.SyntheticRunConfig(), overfitting.NewFakeVerdictStore())
	if err != nil {
		return err
	}

	fmt.Println("strategy-optimizer: synthetic self-check (built-in fixtures, nothing persisted)")
	fmt.Println()
	printRun(result, "would-be status")
	return nil
}

// generateAgainstDB executes one database-backed run: migrations (additive,
// idempotent), loader, validation pipeline, statistics, rules, gate, and --
// unless dry-run -- persistence of the verdicts and proposals.
func generateAgainstDB(fromFlag, toFlag string, dryRun bool) error {
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

	db, err := openDB()
	if err != nil {
		return err
	}
	if err := migrate(db); err != nil {
		return err
	}

	input, outcomes, err := stratopt.LoadRunInput(db, from, to)
	if err != nil {
		return err
	}
	fmt.Printf("loaded %d joined training rows in window [%s, %s)\n\n",
		len(input.Rows), from.Format(time.RFC3339), to.Format(time.RFC3339))

	cfg := stratopt.RunConfig{
		Now:            now,
		Optimizer:      stratopt.DefaultOptimizerConfig(),
		PipelineConfig: optvalidation.DefaultConfig(),
	}

	var verdicts overfitting.VerdictStore = overfitting.NewGormVerdictStore(db)
	statusLabel := "persisted status"
	if dryRun {
		verdicts = overfitting.NewFakeVerdictStore()
		statusLabel = "would-be status"
	}

	result, err := stratopt.GenerateRun(input, outcomes, cfg, verdicts)
	if err != nil {
		return err
	}

	if !dryRun {
		if err := stratopt.PersistRun(db, result.RunID, result.ProposalRows()); err != nil {
			return err
		}
	} else {
		fmt.Println("dry run: nothing persisted")
		fmt.Println()
	}

	printRun(result, statusLabel)
	return nil
}

// printRun prints the pipeline gating summary, every group's statistics and
// eligibility (insufficient-data groups included), and each candidate with
// its rationale, per-check gate verdict, and resulting status.
func printRun(result *stratopt.RunResult, statusLabel string) {
	g := result.Gating
	fmt.Printf("validation pipeline: input=%d timestamp_violations=%d regime_dropped=%d fidelity_dropped=%d clean=%d gated_outcomes=%d\n\n",
		g.InputRows, g.TimestampViolations, g.DroppedByRegime, g.DroppedByFidelity, g.CleanRows, g.GatedOutcomes)

	fmt.Printf("groups (%d):\n", len(result.Groups))
	for _, gr := range result.Groups {
		eligibility := "eligible"
		if !gr.Eligible {
			eligibility = "insufficient data -- no candidates generated"
		}
		fmt.Printf("  %s/%s: decided=%d [%s]\n", gr.StrategyID, gr.Regime, gr.DecidedCount, eligibility)
		fmt.Printf("    weighted_ev=%.6f win_rate=%.4f loss_rate=%.4f avg_win=%.6f avg_loss=%.6f\n",
			gr.Stats.WeightedEV, gr.Stats.WinRate, gr.Stats.LossRate, gr.Stats.AvgWinPct, gr.Stats.AvgLossPct)
		fmt.Printf("    quick_stop_share=%.4f timeout_share=%.4f fast_target_share=%.4f\n",
			gr.Stats.QuickStopShareOfLosses, gr.Stats.TimeoutShareOfLosses, gr.Stats.FastTargetShareOfWins)
	}
	fmt.Println()

	if len(result.Proposals) == 0 {
		fmt.Println("zero proposals generated -- a normal outcome (no group met any rule threshold).")
		return
	}

	fmt.Printf("proposals (%d) -- run %s:\n\n", len(result.Proposals), result.RunID)
	for _, p := range result.Proposals {
		fmt.Printf("proposal %s: %s/%s %s %+.0f%% (rule %s)\n",
			p.Proposal.ID, p.Proposal.StrategyID, p.Proposal.Regime,
			p.Proposal.Parameter, p.Proposal.AdjustmentPct*100, p.Proposal.Rule)
		fmt.Printf("  rationale: %s\n", p.Proposal.Rationale)
		fmt.Printf("  gate verdict: %s\n", passFail(p.Verdict.Passed))
		printChecks(p.Verdict.Checks)
		fmt.Printf("  %s: %s\n\n", statusLabel, p.Proposal.Status)
	}
	fmt.Println(recommendationNote)
}

func printChecks(checks []overfitting.CheckResult) {
	for _, c := range checks {
		fmt.Printf("    [%s] %s\n", passFail(c.Passed), c.Name)
		fmt.Printf("      observed:  %s\n", formatValues(c.Observed))
		fmt.Printf("      threshold: %s\n", formatValues(c.Threshold))
		fmt.Printf("      detail:    %s\n", c.Detail)
	}
}

func listCmd(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	status := fs.String("status", "", "status filter (default pending_review -- the review queue; rejected_by_gate rows appear only under this explicit filter)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := openDB()
	if err != nil {
		return err
	}
	if err := migrate(db); err != nil {
		return err
	}

	effective := *status
	if effective == "" {
		effective = stratopt.StatusPendingReview
	}

	proposals, err := stratopt.ListProposals(db, *status)
	if err != nil {
		return err
	}

	if len(proposals) == 0 {
		fmt.Printf("no %s strategy proposals exist -- a clean outcome. Run `task optimizer:propose -- --db` to generate.\n", effective)
		return nil
	}

	fmt.Printf("strategy proposals (status=%s):\n", effective)
	for _, p := range proposals {
		fmt.Printf("%s  created=%s  %s/%s  %s %+.0f%%  rule=%s  status=%s\n",
			p.ID, p.CreatedAt.Format(time.RFC3339), p.StrategyID, p.Regime,
			p.Parameter, p.AdjustmentPct*100, p.Rule, p.Status)
	}
	fmt.Println()
	fmt.Println(recommendationNote)
	return nil
}

func showCmd(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	idFlag := fs.String("id", "", "proposal id (uuid) to inspect (required)")
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
	if err := migrate(db); err != nil {
		return err
	}

	p, err := stratopt.GetProposal(db, id)
	if err != nil {
		return err
	}

	fmt.Printf("proposal %s\n", p.ID)
	fmt.Printf("  created:    %s (run %s)\n", p.CreatedAt.Format(time.RFC3339), p.RunID)
	fmt.Printf("  group:      %s/%s\n", p.StrategyID, p.Regime)
	fmt.Printf("  change:     %s %+.0f%% (relative), rule %s\n", p.Parameter, p.AdjustmentPct*100, p.Rule)
	fmt.Printf("  status:     %s\n", p.Status)
	if p.DecidedAt != nil {
		via := ""
		if p.DecidedVia != nil {
			via = *p.DecidedVia
		}
		fmt.Printf("  decided:    %s via %s\n", p.DecidedAt.Format(time.RFC3339), via)
	}
	fmt.Printf("  rationale:  %s\n", p.Rationale)

	if evidence, err := stratopt.UnmarshalProposalEvidence(p.EvidenceJSON); err == nil {
		fmt.Println("  evidence:")
		fmt.Printf("    weighted_ev=%.6f win_rate=%.4f loss_rate=%.4f avg_win=%.6f avg_loss=%.6f\n",
			evidence.WeightedEV, evidence.WeightedWinRate, evidence.WeightedLossRate,
			evidence.AvgWinPct, evidence.AvgLossPct)
		fmt.Printf("    quick_stop_share=%.4f timeout_share=%.4f fast_target_share=%.4f trigger_share=%.4f\n",
			evidence.QuickStopShareOfLosses, evidence.TimeoutShareOfLosses,
			evidence.FastTargetShareOfWins, evidence.TriggerShare)
		fmt.Printf("    decided_samples=%d\n", evidence.DecidedSamples)
		gs := evidence.GatingSummary
		fmt.Printf("    gating: input=%d timestamp_violations=%d regime_dropped=%d fidelity_dropped=%d clean=%d gated_outcomes=%d\n",
			gs.InputRows, gs.TimestampViolations, gs.DroppedByRegime, gs.DroppedByFidelity, gs.CleanRows, gs.GatedOutcomes)
	} else {
		fmt.Printf("  evidence:   <unparseable: %v>\n", err)
	}

	var verdictRow overfitting.OverfittingVerdict
	if err := db.First(&verdictRow, "id = ?", p.VerdictID).Error; err != nil {
		fmt.Printf("  gate verdict %s: <not found: %v>\n", p.VerdictID, err)
	} else {
		fmt.Printf("  gate verdict: %s (computed %s, library %s)\n",
			passFail(verdictRow.Passed), verdictRow.ComputedAt.Format(time.RFC3339), verdictRow.LibraryVersion)
		printChecks(verdictRow.ChecksJSON)
	}

	fmt.Println()
	fmt.Println(recommendationNote)
	return nil
}

func decideCmd(args []string) error {
	fs := flag.NewFlagSet("decide", flag.ExitOnError)
	idFlag := fs.String("id", "", "proposal id (uuid) to decide (required)")
	decision := fs.String("decision", "", "accepted or dismissed (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *idFlag == "" {
		return errors.New("--id is required")
	}
	if *decision == "" {
		return errors.New("--decision is required (accepted or dismissed)")
	}
	id, err := uuid.Parse(*idFlag)
	if err != nil {
		return fmt.Errorf("invalid --id %q: %w", *idFlag, err)
	}

	db, err := openDB()
	if err != nil {
		return err
	}
	if err := migrate(db); err != nil {
		return err
	}

	p, err := stratopt.DecideProposal(db, id, *decision, "cli", time.Now().UTC())
	if err != nil {
		return err
	}

	fmt.Printf("proposal %s decided: %s (decided_at=%s, decided_via=%s)\n",
		p.ID, p.Status, p.DecidedAt.Format(time.RFC3339), *p.DecidedVia)
	fmt.Println("the decision changes only the proposal row -- no configuration or trading behavior was modified.")
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
