package models

type ExecuteCloseTradesResult struct {
	BaseResponseEvent
	Trade *Trade `json:"trade"`
}
