package eventmodels

type BaseResponseEvent struct {
	Meta MetaData `json:"meta"`
}

func (r *BaseResponseEvent) GetMetaData() *MetaData {
	return &r.Meta
}

func (r *BaseResponseEvent) SetMetaData(meta *MetaData) {
	r.Meta = *meta
}
