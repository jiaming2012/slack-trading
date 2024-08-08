package eventmodels

type BacktesterOrder struct {
	Underlying StockSymbol
	Spread     *OptionSpreadContractDTO
	Quantity   int
	Tag        string
	Config     *OptionYAML
}
