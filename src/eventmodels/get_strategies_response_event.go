package eventmodels

import (
	"slack-trading/src/models"
)

type GetStrategiesResponseEvent struct {
	BaseResponseEvent
	Strategies []*models.Strategy `json:"strategies"`
}
