package eventmodels

type PolygonGetV3ReferenceOptionsContractsResponse[T any] struct {
	Results   []T     `json:"results"`
	Status    string  `json:"status"`
	RequestId string  `json:"request_id"`
	NextURL   *string `json:"next_url"`
}
