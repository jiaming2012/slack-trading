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
}

func (m *MetaData) EndProcess(err error) {
	m.RequestError <- err
}
