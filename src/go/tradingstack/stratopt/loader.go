package stratopt

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
)

// LoadRunInput loads a generation run's input from the database, mirroring
// the scanner optimizer's loader pattern: it inner-joins scan_results with
// sim_outcomes (via scan_result_id) for scan results whose scanned_at falls
// in the half-open window [from, to), and loads the three reference tables
// (feature_distributions, simulator_fidelity, strategy_ev_weights) wholesale
// into an optvalidation.Input. It also returns the joined sim outcomes
// themselves -- the strategy optimizer needs the outcome fields
// (pnl_pct, hold_days, exit_reason) that the pipeline's TrainingRow does not
// carry, joined back after gating by SimOutcomeID.
//
// The caller MUST thread the returned Input through optvalidation.Run (via
// GenerateRun) -- stratopt exposes no entry point that generates from raw
// rows. Scan results without a sim outcome are excluded (inner join); a scan
// result with multiple outcomes yields one TrainingRow per pair. Rows are
// ordered by scanned_at then id so identical database states load
// identically.
func LoadRunInput(db *gorm.DB, from, to time.Time) (optvalidation.Input, []tradingstack.SimOutcome, error) {
	var scanResults []tradingstack.ScanResult
	if err := db.
		Where("scanned_at >= ? AND scanned_at < ?", from, to).
		Order("scanned_at ASC, id ASC").
		Find(&scanResults).Error; err != nil {
		return optvalidation.Input{}, nil, fmt.Errorf("LoadRunInput: scan_results query failed: %w", err)
	}

	rows := []optvalidation.TrainingRow{}
	joined := []tradingstack.SimOutcome{}
	if len(scanResults) > 0 {
		ids := make([]uuid.UUID, len(scanResults))
		for i, sr := range scanResults {
			ids[i] = sr.ID
		}

		var outcomes []tradingstack.SimOutcome
		if err := db.
			Where("scan_result_id IN ?", ids).
			Order("simulated_at ASC, id ASC").
			Find(&outcomes).Error; err != nil {
			return optvalidation.Input{}, nil, fmt.Errorf("LoadRunInput: sim_outcomes query failed: %w", err)
		}

		outcomesByScanResult := make(map[uuid.UUID][]tradingstack.SimOutcome, len(outcomes))
		for _, so := range outcomes {
			outcomesByScanResult[so.ScanResultID] = append(outcomesByScanResult[so.ScanResultID], so)
		}

		for _, sr := range scanResults {
			for _, so := range outcomesByScanResult[sr.ID] {
				rows = append(rows, optvalidation.NewTrainingRow(sr, so))
				joined = append(joined, so)
			}
		}
	}

	var baseline []tradingstack.FeatureDistribution
	if err := db.Order("computed_at ASC, id ASC").Find(&baseline).Error; err != nil {
		return optvalidation.Input{}, nil, fmt.Errorf("LoadRunInput: feature_distributions query failed: %w", err)
	}

	var fidelity []tradingstack.SimulatorFidelity
	if err := db.Order("computed_at ASC, id ASC").Find(&fidelity).Error; err != nil {
		return optvalidation.Input{}, nil, fmt.Errorf("LoadRunInput: simulator_fidelity query failed: %w", err)
	}

	var evWeights []tradingstack.StrategyEvWeight
	if err := db.Order("computed_at ASC, id ASC").Find(&evWeights).Error; err != nil {
		return optvalidation.Input{}, nil, fmt.Errorf("LoadRunInput: strategy_ev_weights query failed: %w", err)
	}

	return optvalidation.Input{
		Rows:      rows,
		Baseline:  baseline,
		Fidelity:  fidelity,
		EVWeights: evWeights,
	}, joined, nil
}
