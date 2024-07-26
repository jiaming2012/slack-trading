package eventmodels

import "time"

type PolygonTickDataRequest struct {
	BaseURL   string
	StartDate time.Time
	EndDate   time.Time
	Spread    float64
}
