package eventmodels

type SavedEventParameters struct {
	StreamName    StreamName
	EventName     EventName
	SchemaVersion int
}
