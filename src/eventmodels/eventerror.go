package eventmodels

// todo: change to RequestError
type EventError struct {
	Request interface{}
	Error   error
}
