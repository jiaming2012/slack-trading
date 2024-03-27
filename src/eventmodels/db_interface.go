package eventmodels

type SavedEvent interface {
	GetStreamName() string
	GetEventName() EventName
}
