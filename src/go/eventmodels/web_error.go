package eventmodels

type WebError struct {
	StatusCode int
	Message    string
	Cause      error
}

func (e *WebError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}

	return e.Message
}

func NewWebError(statusCode int, message string, cause error) *WebError {
	return &WebError{
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}
}
