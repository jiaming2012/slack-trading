package eventmodels

type GetStatsResult struct {
	BaseResponseEvent
	Strategies []*GetStatsResultItem `json:"strategies"`
}
