package scanneropt

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// LoadInput loads the optimizer's training input from the database: it
// inner-joins scan_results with sim_outcomes (via scan_result_id) for scan
// results whose scanned_at falls in the half-open window [from, to), and
// loads the three reference tables (feature_distributions,
// simulator_fidelity, strategy_ev_weights) wholesale into an
// optvalidation.Input.
//
// This is the first real (non-synthetic) consumer of the
// optimizer-validation-pipeline: the caller MUST thread the returned Input
// through optvalidation.Run and tune only on the pipeline's Result.Clean --
// scanneropt exposes no entry point that tunes from raw rows.
//
// Scan results without a sim outcome are excluded (inner join); a scan result
// with multiple outcomes yields one TrainingRow per pair. Rows are ordered by
// scanned_at then id so identical database states load identically.
func LoadInput(db *gorm.DB, from, to time.Time) (optvalidation.Input, error) {
	var scanResults []tradingstack.ScanResult
	if err := db.
		Where("scanned_at >= ? AND scanned_at < ?", from, to).
		Order("scanned_at ASC, id ASC").
		Find(&scanResults).Error; err != nil {
		return optvalidation.Input{}, fmt.Errorf("LoadInput: scan_results query failed: %w", err)
	}

	rows := []optvalidation.TrainingRow{}
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
			return optvalidation.Input{}, fmt.Errorf("LoadInput: sim_outcomes query failed: %w", err)
		}

		outcomesByScanResult := make(map[uuid.UUID][]tradingstack.SimOutcome, len(outcomes))
		for _, so := range outcomes {
			outcomesByScanResult[so.ScanResultID] = append(outcomesByScanResult[so.ScanResultID], so)
		}

		for _, sr := range scanResults {
			for _, so := range outcomesByScanResult[sr.ID] {
				rows = append(rows, optvalidation.NewTrainingRow(sr, so))
			}
		}
	}

	var baseline []tradingstack.FeatureDistribution
	if err := db.Order("computed_at ASC, id ASC").Find(&baseline).Error; err != nil {
		return optvalidation.Input{}, fmt.Errorf("LoadInput: feature_distributions query failed: %w", err)
	}

	var fidelity []tradingstack.SimulatorFidelity
	if err := db.Order("computed_at ASC, id ASC").Find(&fidelity).Error; err != nil {
		return optvalidation.Input{}, fmt.Errorf("LoadInput: simulator_fidelity query failed: %w", err)
	}

	var evWeights []tradingstack.StrategyEvWeight
	if err := db.Order("computed_at ASC, id ASC").Find(&evWeights).Error; err != nil {
		return optvalidation.Input{}, fmt.Errorf("LoadInput: strategy_ev_weights query failed: %w", err)
	}

	return optvalidation.Input{
		Rows:      rows,
		Baseline:  baseline,
		Fidelity:  fidelity,
		EVWeights: evWeights,
	}, nil
}

// LoadActiveBaseline resolves the current active scanner config payload: the
// scanner_configs row with the latest created_at (ties broken by id), parsed
// from its config_json. When the table is empty it returns
// scannercfg.DefaultPayload() -- the same "active" rule the
// shadow-config-deployment change pins, so both agree before hot-swap exists.
func LoadActiveBaseline(db *gorm.DB) (scannercfg.Payload, error) {
	var cfg tradingstack.ScannerConfig
	err := db.
		Order("created_at DESC NULLS LAST, id ASC").
		First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return scannercfg.DefaultPayload(), nil
	}
	if err != nil {
		return scannercfg.Payload{}, fmt.Errorf("LoadActiveBaseline: scanner_configs query failed: %w", err)
	}

	payload, err := scannercfg.Parse(cfg.ConfigJSON)
	if err != nil {
		return scannercfg.Payload{}, fmt.Errorf("LoadActiveBaseline: active config %s is unparseable: %w", cfg.ID, err)
	}
	return payload, nil
}
