package eventmodels

import (
	"sync"

	"github.com/google/uuid"
)

type MetaData struct {
	RequestError      chan error    `json:"-"`
	Mutex             *sync.Mutex   `json:"-"`
	RequestID         uuid.UUID     `json:"request_id"`
	IsExternalRequest bool          `json:"-"`
	EventStreamID     EventStreamID `json:"event_stream_id"`
}

func (m *MetaData) SetEventStreamID(id EventStreamID) {
	m.EventStreamID = id
}

func (m *MetaData) GetEventStreamID() EventStreamID {
	return m.EventStreamID
}

func (m *MetaData) EndProcess(err error) {
	m.RequestError <- err
}
