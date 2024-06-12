package eventmodels

type GetOptionAlertResponseEvent struct {
	BaseResponseEvent
	Alerts []OptionAlert `json:"alerts"`
}
