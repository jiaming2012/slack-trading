package eventmodels

type ExecuteCloseTradesResult struct {
	BaseResponseEvent2
	Trade *Trade `json:"trade"`
}
