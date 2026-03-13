package eventmodels

type PolygonAggregateBarsResponse struct {
	Results   []PolygonAggregateBar `json:"results"`
	Status    string                `json:"status"`
	RequestId string                `json:"request_id"`
}
