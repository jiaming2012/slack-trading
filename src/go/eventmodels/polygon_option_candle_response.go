package eventmodels

type PolygonOptionCandleResponse struct {
	Ticker       string             `json:"ticker"`
	QueryCount   int                `json:"queryCount"`
	ResultsCount int                `json:"resultsCount"`
	Adjusted     bool               `json:"adjusted"`
	Results      []PolygonCandleDTO `json:"results"`
}
