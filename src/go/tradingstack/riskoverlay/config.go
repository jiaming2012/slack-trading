package riskoverlay

import (
	"errors"
	"fmt"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

// ErrInvalidRiskLimits is the sentinel returned when a configured limit is
// invalid: a negative currency/exposure limit, or a percentage limit outside
// the range 0–100.
var ErrInvalidRiskLimits = errors.New("riskoverlay: invalid risk limits")

// DefaultRiskLimits are the documented, deliberately permissive defaults. They
// are chosen so an unconfigured gate observes without blocking: the exposure
// and capital caps are effectively unlimited, the percentage caps sit at their
// no-op maximum (100), and crowded-entry rejection is off. Enforcement is only
// meaningful once an operator narrows these values.
//
//	MaxGrossExposure          1e15   (effectively unlimited)
//	MaxNetExposure            1e15   (effectively unlimited)
//	MaxSectorConcentrationPct 100.0  (a single sector may be the whole book)
//	MaxDrawdownPct            100.0  (breaker never trips)
//	DeployableCapital         1e15   (effectively unlimited)
//	RejectCrowdedEntries      false  (crowding consumption off)
var DefaultRiskLimits = RiskLimits{
	MaxGrossExposure:          1e15,
	MaxNetExposure:            1e15,
	MaxSectorConcentrationPct: 100.0,
	MaxDrawdownPct:            100.0,
	DeployableCapital:         1e15,
	RejectCrowdedEntries:      false,
}

// riskOverlayYAML mirrors the on-disk YAML shape under the riskOverlay block.
// Pointer fields distinguish "absent" (fall back to the default) from an
// explicit zero.
//
//	riskOverlay:
//	  max_gross_exposure: 100000
//	  max_net_exposure: 90000
//	  max_sector_concentration_pct: 50
//	  max_drawdown_pct: 5
//	  deployable_capital: 100000
//	  reject_crowded_entries: true
type riskOverlayYAML struct {
	RiskOverlay *struct {
		MaxGrossExposure          *float64 `yaml:"max_gross_exposure"`
		MaxNetExposure            *float64 `yaml:"max_net_exposure"`
		MaxSectorConcentrationPct *float64 `yaml:"max_sector_concentration_pct"`
		MaxDrawdownPct            *float64 `yaml:"max_drawdown_pct"`
		DeployableCapital         *float64 `yaml:"deployable_capital"`
		RejectCrowdedEntries      *bool    `yaml:"reject_crowded_entries"`
	} `yaml:"riskOverlay"`
}

// LoadRiskLimits parses a riskOverlay YAML block from raw bytes, applying
// DefaultRiskLimits for any absent block or field, and validating the result.
// A missing block yields the documented defaults with no error. An invalid
// value yields ErrInvalidRiskLimits and no usable RiskLimits.
func LoadRiskLimits(raw []byte) (RiskLimits, error) {
	limits := DefaultRiskLimits

	var parsed riskOverlayYAML
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return RiskLimits{}, fmt.Errorf("riskoverlay: failed to parse config: %w", err)
	}

	if parsed.RiskOverlay != nil {
		blk := parsed.RiskOverlay
		if blk.MaxGrossExposure != nil {
			limits.MaxGrossExposure = *blk.MaxGrossExposure
		}
		if blk.MaxNetExposure != nil {
			limits.MaxNetExposure = *blk.MaxNetExposure
		}
		if blk.MaxSectorConcentrationPct != nil {
			limits.MaxSectorConcentrationPct = *blk.MaxSectorConcentrationPct
		}
		if blk.MaxDrawdownPct != nil {
			limits.MaxDrawdownPct = *blk.MaxDrawdownPct
		}
		if blk.DeployableCapital != nil {
			limits.DeployableCapital = *blk.DeployableCapital
		}
		if blk.RejectCrowdedEntries != nil {
			limits.RejectCrowdedEntries = *blk.RejectCrowdedEntries
		}
	}

	if err := ValidateRiskLimits(limits); err != nil {
		return RiskLimits{}, err
	}

	return limits, nil
}

// ValidateRiskLimits rejects negative currency/exposure limits and percentage
// limits outside 0–100, returning ErrInvalidRiskLimits.
func ValidateRiskLimits(l RiskLimits) error {
	if l.MaxGrossExposure < 0 {
		return fmt.Errorf("%w: max_gross_exposure must be >= 0, got %.4f", ErrInvalidRiskLimits, l.MaxGrossExposure)
	}
	if l.MaxNetExposure < 0 {
		return fmt.Errorf("%w: max_net_exposure must be >= 0, got %.4f", ErrInvalidRiskLimits, l.MaxNetExposure)
	}
	if l.DeployableCapital < 0 {
		return fmt.Errorf("%w: deployable_capital must be >= 0, got %.4f", ErrInvalidRiskLimits, l.DeployableCapital)
	}
	if l.MaxSectorConcentrationPct < 0 || l.MaxSectorConcentrationPct > 100 {
		return fmt.Errorf("%w: max_sector_concentration_pct must be within 0–100, got %.4f", ErrInvalidRiskLimits, l.MaxSectorConcentrationPct)
	}
	if l.MaxDrawdownPct < 0 || l.MaxDrawdownPct > 100 {
		return fmt.Errorf("%w: max_drawdown_pct must be within 0–100, got %.4f", ErrInvalidRiskLimits, l.MaxDrawdownPct)
	}
	return nil
}

// LoadRiskLimitsFromFile reads and parses the risk-overlay config file at path.
// A file that cannot be read yields the documented defaults with no error, so
// an absent config never blocks startup; a present-but-invalid config yields
// ErrInvalidRiskLimits.
func LoadRiskLimitsFromFile(path string) (RiskLimits, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return DefaultRiskLimits, nil
	}
	return LoadRiskLimits(raw)
}

// ResolveConfigPath resolves the risk-overlay config file path: the
// RISK_OVERLAY_CONFIG_PATH env var if set, else
// ${TRADING_PROJECT_DIR}/src/go/risk-overlay-config.yaml, mirroring the
// existing OPTIONS_CONFIG_PATH / FEED_HEALTH_CONFIG_PATH convention.
func ResolveConfigPath() (string, error) {
	if p := os.Getenv("RISK_OVERLAY_CONFIG_PATH"); p != "" {
		return p, nil
	}
	projectDir := os.Getenv("TRADING_PROJECT_DIR")
	if projectDir == "" {
		return "", fmt.Errorf("riskoverlay: neither RISK_OVERLAY_CONFIG_PATH nor TRADING_PROJECT_DIR is set")
	}
	return path.Join(projectDir, "src", "go", "risk-overlay-config.yaml"), nil
}
