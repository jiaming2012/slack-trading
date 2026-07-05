package models

type GetOptionAlertResponseEvent struct {
	BaseResponseEvent
	Alerts []OptionAlert `json:"alerts"`
}
