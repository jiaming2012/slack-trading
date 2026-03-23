package eventmodels

type AggregateResult[T any] struct {
	QueryCount   int            `json:"query_count"`
	ResultsCount int            `json:"results_count"`
	Results      []T            `json:"results"`
	GetNextURL   func() *string `json:"-"`
}
