package feedhealth

import (
	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// TagScanResult sets result.FeedHealth to the string form of status
// ("healthy", "degraded", or "stale"), stamping the computed feed health
// onto a scan result at scan time so downstream EV/optimizer components can
// filter or downweight scans from degraded-feed periods.
func TagScanResult(result *tradingstack.ScanResult, status FeedHealthStatus) {
	s := status.String()
	result.FeedHealth = &s
}
