package eventmodels

type OptionSpreadContractDTO struct {
	Description       string       `json:"description"`
	Type              OptionType   `json:"type"`
	DebitPaid         *float64     `json:"debit_paid"`
	CreditReceived    *float64     `json:"credit_received"`
	LongOptionSymbol  OptionSymbol `json:"longOptionSymbol"`
	ShortOptionSymbol OptionSymbol `json:"shortOptionSymbol"`
	Stats             OptionStats  `json:"stats"`
}
