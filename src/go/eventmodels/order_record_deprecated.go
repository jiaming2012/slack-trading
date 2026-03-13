package eventmodels

type OrderRecordDeprecated struct {
	Underlying StockSymbol
	Spread     *OptionSpreadContractDTO
	Quantity   float64
	Tag        string
	Config     *OptionYAML
}
