package models

type CreateSignalResponseEvent struct {
	BaseResponseEvent
	Name string `json:"name"`
}
