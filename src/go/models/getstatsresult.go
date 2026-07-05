package models

type GetStatsResult struct {
	BaseResponseEvent
	Strategies []*GetStatsResultItem `json:"strategies"`
}
