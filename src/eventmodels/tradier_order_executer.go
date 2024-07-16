package eventmodels

type TradierOrderExecuter struct {
	Url         string
	BearerToken string
	DryRun      bool
}

func NewTradierOrderExecuter(url, bearerToken string, dryRun bool) *TradierOrderExecuter {
	return &TradierOrderExecuter{Url: url, BearerToken: bearerToken, DryRun: dryRun}
}
