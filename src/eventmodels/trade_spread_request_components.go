package eventmodels

type TradeSpreadRequestComponents struct {
	Tag            string
	Spread         *OptionSpreadContractDTO
	RequestedPrice float64
}
