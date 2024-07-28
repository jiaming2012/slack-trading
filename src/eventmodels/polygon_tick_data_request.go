package eventmodels

import "time"

type PolygonOptionTickDataRequest struct {
	BaseURL   string
	StartDate time.Time
	EndDate   time.Time
	Spread    float64
}
