package eventmodels

import (
	"github.com/jiaming2012/slack-trading/src/models"
)

type GetStrategiesResponseEvent struct {
	BaseResponseEvent
	Strategies []*models.Strategy `json:"strategies"`
}
