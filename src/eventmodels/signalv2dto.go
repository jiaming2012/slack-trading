package eventmodels

import "time"

type SignalV2DTO struct {
	Name        string    `json:"name"`
	IsSatisfied bool      `json:"isSatisfied"`
	LastUpdated time.Time `json:"lastUpdated"`
}
