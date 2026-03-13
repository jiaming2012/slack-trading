package eventmodels

type PolygonAggregateBar struct {
	Volume         int     `json:"v"`
	VolumeWeighted float64 `json:"vw"`
	Open           float64 `json:"o"`
	Close          float64 `json:"c"`
	High           float64 `json:"h"`
	Low            float64 `json:"l"`
	Time           int     `json:"t"`
	Count          int     `json:"n"`
}
