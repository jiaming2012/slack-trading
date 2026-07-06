package telemetry

// Default is the process-wide registry, non-nil after Init().
var Default *Registry

// Metric instruments -- nil until Init() is called. Recording through a nil
// instrument is a safe no-op, so call sites need no guards.
var (
	OrdersPlaced      *Counter
	OrdersFilled      *Counter
	OrdersRejected    *Counter
	CandlesProcessed  *Counter
	SignalsGenerated  *Counter
	SignalsConsumed   *Counter
	ErrorsTotal       *Counter
	ActivePlaygrounds *Gauge
	OpenOrders        *Gauge
	UptimeSeconds     *Gauge

	// Anomaly-guard / kill-switch instruments (wire-anomaly-guard-feeds).
	// GuardObservations and GuardTrips carry a {guard} label naming the guard;
	// HaltEngaged is 0/1, set on every halt-controller transition and on
	// startup state restore.
	GuardObservations *Counter
	GuardTrips        *Counter
	HaltEngaged       *Gauge

	// Companion-stop / deferred-auto-close instruments (wire-companion-stops).
	// DeferredAutoCloses carries a {playground_id} label and reports how many
	// option auto-closes are currently deferred by an engaged halt (open
	// exposure); the companion-stop counters track broker-held protective
	// stops placed on live entry fills and placement failures (each failure is
	// a live position left UNPROTECTED).
	DeferredAutoCloses    *Gauge
	CompanionStopsPlaced  *Counter
	CompanionStopFailures *Counter

	// Portfolio risk overlay instruments (wire-risk-overlay-state).
	// RiskOverlayDegraded counts every fail-permissive permit or
	// unknown-sector evaluation, labeled {reason}: the gate stayed permissive
	// while (partially) blind. RiskOverlayRejections counts rejected orders
	// once per breached limit family, labeled {limit_type}.
	// RiskOverlayEnabled is 0/1 gate enablement; RiskOverlayEvFamilyActive is
	// 0/1 whether the last EV-weight lookup returned a non-empty set (0 =
	// strategy-allocation family pinned inactive). A DISABLED gate records
	// nothing on any of these (permissive-blind).
	RiskOverlayDegraded       *Counter
	RiskOverlayRejections     *Counter
	RiskOverlayEnabled        *Gauge
	RiskOverlayEvFamilyActive *Gauge

	// Fidelity-monitor instruments (continuous-fidelity-monitoring).
	// FidelityRuns counts scheduled fidelity runs labeled {status}
	// (ok|breach|no_data|error). FidelityDriftScore and
	// FidelityWithinTolerance (1.0 within / 0.0 breaching) carry a
	// {strategy_id} label and are updated on every result-producing run. They
	// are observability for task fidelity:status and snapshot history — the
	// fidelity_drift alert rule is fed by the AlertEngine's explicit snapshot
	// (ReportFidelity), never by these gauges (design D3: the registry has no
	// series deletion, so a stale gauge cannot distinguish "still breaching"
	// from "no longer evaluated").
	FidelityRuns            *Counter
	FidelityDriftScore      *Gauge
	FidelityWithinTolerance *Gauge
)

// Init constructs the registry and all instruments. It is self-contained:
// no SDK, provider, or network setup is required beforehand, and recording
// is safe immediately — persistence picks up whatever is in the registry
// once the snapshot writer starts.
func Init() {
	Default = NewRegistry()
	Heartbeats = NewHeartbeatTracker()

	OrdersPlaced = Default.Counter("grodt.orders.placed")
	OrdersFilled = Default.Counter("grodt.orders.filled")
	OrdersRejected = Default.Counter("grodt.orders.rejected")
	CandlesProcessed = Default.Counter("grodt.candles.processed")
	SignalsGenerated = Default.Counter("grodt.signals.generated")
	SignalsConsumed = Default.Counter("grodt.signals.consumed")
	ErrorsTotal = Default.Counter("grodt.errors.total")
	ActivePlaygrounds = Default.Gauge("grodt.heartbeat.active_playgrounds")
	OpenOrders = Default.Gauge("grodt.heartbeat.open_orders")
	UptimeSeconds = Default.Gauge("grodt.heartbeat.uptime")
	GuardObservations = Default.Counter("safety_guard_observations_total")
	GuardTrips = Default.Counter("safety_guard_trips_total")
	HaltEngaged = Default.Gauge("safety_halt_engaged")
	DeferredAutoCloses = Default.Gauge(MetricDeferredAutoCloses)
	CompanionStopsPlaced = Default.Counter("safety_companion_stops_placed_total")
	CompanionStopFailures = Default.Counter("safety_companion_stop_failures_total")
	RiskOverlayDegraded = Default.Counter(MetricRiskOverlayDegraded)
	RiskOverlayRejections = Default.Counter(MetricRiskOverlayRejections)
	RiskOverlayEnabled = Default.Gauge(MetricRiskOverlayEnabled)
	RiskOverlayEvFamilyActive = Default.Gauge(MetricRiskOverlayEvFamilyActive)
	FidelityRuns = Default.Counter("grodt.fidelity.runs.total")
	FidelityDriftScore = Default.Gauge("grodt.fidelity.drift_score")
	FidelityWithinTolerance = Default.Gauge("grodt.fidelity.within_tolerance")
}

// Risk-overlay metric names, exported because the alert engine reads these
// series back out of registry snapshots to drive the riskoverlay alert rules.
const (
	MetricRiskOverlayDegraded       = "grodt.riskoverlay.degraded"
	MetricRiskOverlayRejections     = "grodt.riskoverlay.rejections"
	MetricRiskOverlayEnabled        = "grodt.riskoverlay.enabled"
	MetricRiskOverlayEvFamilyActive = "grodt.riskoverlay.ev_family_active"
)

// MetricDeferredAutoCloses is exported by name because the alert engine reads
// the gauge back out of registry snapshots to drive the deferred-auto-close
// alert rule.
const MetricDeferredAutoCloses = "safety_deferred_auto_closes"

// GuardLabel is the standard {guard} dimension for the guard counters.
func GuardLabel(guardName string) Label {
	return Label{Key: "guard", Value: guardName}
}

// ShouldEmitOrderTelemetry returns true if the verbose per-order log lines
// should be emitted for the given playground environment. Metrics are
// recorded in every mode (tagged by mode); only the log noise is gated to
// live and reconcile playgrounds.
func ShouldEmitOrderTelemetry(env string) bool {
	return env == "live" || env == "reconcile"
}

// ClientIDOrEmpty safely dereferences a *string to a string for client_id.
// Returns empty string if the pointer is nil.
func ClientIDOrEmpty(clientID *string) string {
	if clientID != nil {
		return *clientID
	}
	return ""
}

// PlaygroundAttrs returns the standard label set for a playground-scoped
// series: mode, account_type, and client_id dimensions.
func PlaygroundAttrs(env string, accountType string, clientID string) []Label {
	return []Label{
		{Key: "mode", Value: env},
		{Key: "account_type", Value: accountType},
		{Key: "client_id", Value: clientID},
	}
}
