package eventmodels

type BaseResponseEvent2 struct {
	Meta MetaData `json:"meta"`
}

func (r *BaseResponseEvent2) GetMetaData() MetaData {
	return r.Meta
}

func (r *BaseResponseEvent2) SetMetaData(meta *MetaData) {
	r.Meta = *meta
}
