package models

import (
)

type GetStrategiesResponseEvent struct {
	BaseResponseEvent
	Strategies []*Strategy `json:"strategies"`
}
