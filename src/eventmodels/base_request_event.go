package eventmodels

type BaseRequestEvent struct {
	Meta *MetaData `json:"meta"`
}

func (r *BaseRequestEvent) GetMetaData() *MetaData {
	return r.Meta
}

func (r *BaseRequestEvent) SetMetaData(meta *MetaData) {
	r.Meta = meta
}
