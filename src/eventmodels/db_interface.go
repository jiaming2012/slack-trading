package eventmodels

type DBInterface interface {
	GetStreamName() string
	GetEventName() EventName
}
