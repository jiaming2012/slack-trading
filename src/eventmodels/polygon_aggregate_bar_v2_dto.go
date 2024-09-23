package eventmodels

type PolygonAggregateBarV2DTO struct {
	Volume    float64 `json:"Volume"`
	VWAP      float64 `json:"-"`
	Open      float64 `json:"Open"`
	Close     float64 `json:"Close"`
	High      float64 `json:"High"`
	Low       float64 `json:"Low"`
	Timestamp string  `json:"Datetime"`
}
