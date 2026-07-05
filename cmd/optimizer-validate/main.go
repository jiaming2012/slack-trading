// Command optimizer-validate runs the optimizer validation pipeline (the
// five-stage pre-training data gate over scan_results/sim_outcomes) and
// prints a per-stage summary. It exits non-zero only when the pipeline
// itself returns an internal error -- never merely because rows were
// dropped or drift was flagged.
package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
)

func main() {
	synthetic := flag.Bool("synthetic", true, "run against built-in synthetic fixtures (no database or network required)")
	flag.Parse()

	if !*synthetic {
		log.Fatalf("optimizer-validate: only --synthetic=true is currently supported; a real GORM-backed loader is out of scope for this change")
	}

	input := optvalidation.SyntheticInput()
	cfg := optvalidation.DefaultConfig()

	result, err := optvalidation.Run(input, cfg)
	if err != nil {
		log.Errorf("optimizer-validate: pipeline run failed: %v", err)
		os.Exit(1)
	}

	printSummary(result)
}

func printSummary(result optvalidation.Result) {
	fmt.Printf("optimizer-validate: kept=%d timestamp_violations=%d regime_dropped=%d fidelity_dropped=%d drifted_features=%v\n",
		len(result.Clean),
		len(result.Violations),
		len(result.DroppedByRegime),
		len(result.DroppedByFidelity),
		result.DistributionReport.Drifted,
	)

	for _, row := range result.Clean {
		fmt.Printf("  strategy=%s ticker=%s regime=%s ev_weight=%.4f\n",
			row.StrategyID, row.Ticker, row.RegimeTag, row.EVWeight)
	}
}
