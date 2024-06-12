package eventmodels

type OptionContracts []*OptionContractV1

func (c OptionContracts) GetListOfExpirations() []string {
	expirationsMap := make(map[string]struct{})
	for _, contract := range c {
		expirationsMap[contract.Expiration.Format("2006-01-02")] = struct{}{}
	}

	expirations := make([]string, 0, len(expirationsMap))
	for expiration := range expirationsMap {
		expirations = append(expirations, expiration)
	}

	return expirations
}

func (c OptionContracts) GetListOfUnderlyingSymbols() []StockSymbol {
	symbolsMap := make(map[StockSymbol]struct{})
	for _, contract := range c {
		symbolsMap[contract.UnderlyingSymbol] = struct{}{}
	}

	symbols := make([]StockSymbol, 0, len(symbolsMap))
	for symbol := range symbolsMap {
		symbols = append(symbols, symbol)
	}

	return symbols
}
