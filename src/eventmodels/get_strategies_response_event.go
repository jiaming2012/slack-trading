package eventmodels

import (
	"slack-trading/src/models"
)

type GetStrategiesResponseEvent struct {
	BaseResponseEvent2
	Strategies []*models.Strategy `json:"strategies"`
}
