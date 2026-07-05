package models

type FetchDataFunc[T any] func(url, apiKey string) (*AggregateResult[T], error)
