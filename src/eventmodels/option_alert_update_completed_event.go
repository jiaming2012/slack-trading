package eventmodels

import "github.com/google/uuid"

type OptionAlertUpdateCompletedEvent struct{}

func (e *OptionAlertUpdateCompletedEvent) GetMetaData() *MetaData {
	return nil
}

func (e *OptionAlertUpdateCompletedEvent) GetRequestID() uuid.UUID {
	return uuid.Nil
}
