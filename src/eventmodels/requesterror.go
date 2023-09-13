package eventmodels

import "github.com/google/uuid"

type RequestError interface {
	error
	RequestID() uuid.UUID
}

type requestError struct {
	ID  uuid.UUID
	Err error
}

func (e requestError) Error() string {
	return e.Err.Error()
}

func (e requestError) RequestID() uuid.UUID {
	return e.ID
}

func NewRequestError(id uuid.UUID, err error) RequestError {
	return &requestError{
		ID:  id,
		Err: err,
	}
}
