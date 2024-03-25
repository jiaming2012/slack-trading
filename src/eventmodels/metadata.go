package eventmodels

import (
	log "github.com/sirupsen/logrus"
)

type MetaData struct {
	ParentMeta   *MetaData          `json:"-"`
	RequestError chan RequestError2 `json:"-"`
}

func NewMetaData(parentMeta *MetaData) *MetaData {
	return &MetaData{
		ParentMeta:   parentMeta,
		RequestError: parentMeta.RequestError,
	}
}

func (m *MetaData) EndProcess(req interface{}, err error) {
	if m == nil {
		log.Warnf("MetaData.EndProcess: m is nil, type=%T", req)
		return
	}

	// pass the error up the parent chain
	if m.ParentMeta != nil {
		m.ParentMeta.EndProcess(req, err)
	}

	m.RequestError <- RequestError2{
		Request: req,
		Error:   err,
	}
}
