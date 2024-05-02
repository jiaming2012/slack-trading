package eventmodels

import (
	"sync"

	"github.com/google/uuid"
)

type MetaData struct {
	RequestError      chan error  `json:"-"`
	Mutex             *sync.Mutex `json:"-"`
	RequestID         uuid.UUID   `json:"request_id"`
	IsExternalRequest bool        `json:"-"`
	StreamID          uuid.UUID   `json:"stream_id"`
	StreamVersion     int         `json:"stream_version"`
}

func (m *MetaData) SetSchemaVersion(version int) {
	m.StreamVersion = version
}

func (m *MetaData) GetSchemaVersion() int {
	return m.StreamVersion
}

func (m *MetaData) SetEventStreamID(id EventStreamID) {
	m.StreamID = uuid.UUID(id)
}

func (m *MetaData) GetEventStreamID() EventStreamID {
	return EventStreamID(m.StreamID)
}

func (m *MetaData) EndProcess(err error) {
	m.RequestError <- err
}
