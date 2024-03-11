package eventmodels

type ExitConditionsSatisfied struct {
	PriceLevel      *PriceLevel
	PriceLevelIndex int
	PercentClose    ClosePercent
	Reason          string
}
