package eventmodels

type TradierOrderExecuter struct {
	Url             string
	BearerToken     string
	DryRun          bool
	PositionFetcher func() ([]TradierPositionDTO, error)
}

func NewTradierOrderExecuter(url, bearerToken string, dryRun bool, positionFetcher func() ([]TradierPositionDTO, error)) *TradierOrderExecuter {
	return &TradierOrderExecuter{Url: url, BearerToken: bearerToken, DryRun: dryRun, PositionFetcher: positionFetcher}
}
