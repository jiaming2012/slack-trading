package eventmodels

type WebError struct {
	StatusCode int
	Message    string
}

func (e *WebError) Error() string {
	return e.Message
}

func NewWebError(statusCode int, message string) *WebError {
	return &WebError{
		StatusCode: statusCode,
		Message:    message,
	}
}