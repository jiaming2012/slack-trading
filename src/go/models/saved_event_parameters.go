package models

type SavedEventParameters struct {
	StreamName    StreamName
	EventName     EventName
	SchemaVersion int
}
