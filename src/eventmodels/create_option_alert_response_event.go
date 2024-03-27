package eventmodels

import (
	"sync"

	"github.com/google/uuid"
)

type CreateOptionAlertResponseEvent struct {
	BaseResponseEvent2
	ID    string `json:"id"`
	mutex *sync.Mutex
}

func (e *CreateOptionAlertResponseEvent) GetMutex() *sync.Mutex {
	return e.mutex
}

func NewCreateOptionAlertResponseEvent(requestID uuid.UUID, id string, mutex *sync.Mutex) *CreateOptionAlertResponseEvent {
	return &CreateOptionAlertResponseEvent{
		BaseResponseEvent2: BaseResponseEvent2{
			Meta: &MetaData{
				RequestID: requestID,
			},
		},
		ID:    id,
		mutex: mutex,
	}
}
