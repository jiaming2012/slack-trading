package eventmodels

type CreateSignalResponseEvent struct {
	BaseResponseEvent2
	Name string `json:"name"`
}

func (r *CreateSignalResponseEvent) GetMetaData() *MetaData {
	return r.Meta
}
