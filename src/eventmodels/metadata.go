package eventmodels

type MetaData struct {
	ParentMeta   *MetaData  `json:"-"`
	RequestError chan error `json:"-"`
}

func NewMetaData(parentMeta *MetaData) *MetaData {
	return &MetaData{
		ParentMeta:   parentMeta,
		RequestError: parentMeta.RequestError,
	}
}

func (m *MetaData) EndProcess(err error) {
	// pass the error up the parent chain
	if m.ParentMeta != nil {
		m.ParentMeta.EndProcess(err)
	}

	m.RequestError <- err
}
