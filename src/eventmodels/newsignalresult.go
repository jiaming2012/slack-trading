package eventmodels

type CreateSignalResponseEvent struct {
	BaseResponseEvent2
	Name string `json:"name"`
}
