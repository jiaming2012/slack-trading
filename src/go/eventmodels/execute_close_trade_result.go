package eventmodels

type ExecuteCloseTradesResult struct {
	BaseResponseEvent
	Trade *Trade `json:"trade"`
}
