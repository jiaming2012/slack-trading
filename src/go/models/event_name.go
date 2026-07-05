package models

type EventName string

func NewSavedEvent(event EventName) EventName {
	return EventName(event + "SavedEventName")
}
