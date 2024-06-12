package eventmodels

type CreateSignalResponseEvent struct {
	BaseResponseEvent
	Name string `json:"name"`
}
