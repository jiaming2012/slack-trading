package eventmodels

type AggregateResult[T any] struct {
	QueryCount   int
	ResultsCount int
	Results      []T
	GetNextURL   func() *string
}
