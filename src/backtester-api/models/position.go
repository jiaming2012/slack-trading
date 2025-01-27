package models

type Position struct {
	Quantity          float64            `json:"quantity"`
	CostBasis         float64            `json:"cost_basis"`
	PL                float64            `json:"pl"`
	MaintenanceMargin float64            `json:"maintenance_margin"`
	CurrentPrice	  float64            `json:"current_price"`
}
