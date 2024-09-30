package models

type Position struct {
	Quantity  float64 `json:"quantity"`
	CostBasis float64 `json:"cost_basis"`
}