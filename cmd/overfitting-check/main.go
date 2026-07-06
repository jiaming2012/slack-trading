// Command overfitting-check runs the shared optimizer overfitting gate over
// built-in synthetic fixtures -- one passing, one failing -- and prints every
// check's name, observed values, thresholds, and pass/fail, plus each overall
// verdict. It requires no database and no network, and exits non-zero only on
// an internal error: the failing fixture's failed verdict is the expected
// demonstration, not an error.
package main

import (
	"flag"
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

func main() {
	synthetic := flag.Bool("synthetic", true, "run against built-in synthetic fixtures (no database or network required)")
	flag.Parse()

	if !*synthetic {
		log.Fatalf("overfitting-check: only --synthetic=true is currently supported; evidence is constructed in memory by the consuming optimizers, not loaded from a database")
	}

	cfg := overfitting.DefaultConfig()

	fixtures := []struct {
		label    string
		evidence overfitting.Evidence
	}{
		{"passing fixture", overfitting.SyntheticEvidencePass()},
		{"failing fixture", overfitting.SyntheticEvidenceFail()},
	}

	for _, fixture := range fixtures {
		verdict, err := overfitting.RunGate(fixture.evidence, cfg)
		if err != nil {
			log.Fatalf("overfitting-check: gate run failed for %s: %v", fixture.label, err)
		}
		printVerdict(fixture.label, verdict)
	}
}

func printVerdict(label string, v overfitting.Verdict) {
	fmt.Printf("%s: proposal=%s kind=%s verdict=%s\n",
		label, v.ProposalID, v.ProposalKind, passFail(v.Passed))
	for _, c := range v.Checks {
		fmt.Printf("  [%s] %s\n", passFail(c.Passed), c.Name)
		fmt.Printf("    observed:  %s\n", formatValues(c.Observed))
		fmt.Printf("    threshold: %s\n", formatValues(c.Threshold))
		fmt.Printf("    detail:    %s\n", c.Detail)
	}
	fmt.Println()
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
