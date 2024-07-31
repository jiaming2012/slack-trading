package eventmodels

type BacktesterOrder struct {
	Underlying StockSymbol
	Spread     *OptionSpreadContractDTO
	Condor     *OptionCondorContractDTO
	Quantity   int
	Tag        string
}
