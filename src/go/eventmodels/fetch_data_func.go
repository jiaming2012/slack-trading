package eventmodels

type FetchDataFunc[T any] func(url, apiKey string) (*AggregateResult[T], error)
