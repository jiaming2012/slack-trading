package eventmodels

type GetOptionAlertResponseEvent struct {
	BaseResponseEvent2
	Alerts []OptionAlert `json:"alerts"`
}
