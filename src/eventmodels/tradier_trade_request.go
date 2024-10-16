package eventmodels

type BacktesterOrder struct {
	Underlying StockSymbol
	Spread     *OptionSpreadContractDTO
	Quantity   float64
	Tag        string
	Config     *OptionYAML
}
