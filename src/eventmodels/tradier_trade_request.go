package eventmodels

type OrderRecord struct {
	Underlying StockSymbol
	Spread     *OptionSpreadContractDTO
	Quantity   float64
	Tag        string
	Config     *OptionYAML
}
