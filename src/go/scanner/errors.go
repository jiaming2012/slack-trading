package scanner

import (
	"errors"
	"fmt"
	"strings"
)

// ErrDataAsOfAfterScannedAt is returned by BuildFeatureVector when the
// derived data_as_of timestamp would land strictly after scanned_at. This is
// the fail-closed path for the Feature Vector Accuracy Rules: no partial
// vector is returned and no scan_results row is written.
var ErrDataAsOfAfterScannedAt = errors.New("scanner: data_as_of must be <= scanned_at")

// ErrFilteredOut is returned by RunScan when a ticker is rejected by Layer 1.
// It carries every failing gate's reason (Layer 1 never short-circuits after
// the first failure), so callers can see the full picture in one error.
type ErrFilteredOut struct {
	Reasons []string
}

func (e ErrFilteredOut) Error() string {
	return fmt.Sprintf("scanner: filtered out by Layer 1: %s", strings.Join(e.Reasons, "; "))
}
