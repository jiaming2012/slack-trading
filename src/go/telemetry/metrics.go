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
