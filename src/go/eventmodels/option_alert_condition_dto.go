package eventmodels

type OptionAlertConditionDTO struct {
	Type      string  `json:"type"`
	Direction string  `json:"direction"`
	Value     float64 `json:"value"`
}
