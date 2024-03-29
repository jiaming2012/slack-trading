package eventmodels

type GetStatsResult struct {
	BaseResponseEvent2
	Strategies []*GetStatsResultItem `json:"strategies"`
}
