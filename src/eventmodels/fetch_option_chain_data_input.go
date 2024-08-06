package eventmodels

type FetchOptionChainDataInput struct {
	OptionContracts  []OptionContractV3
	StockTickItemDTO *StockTickItemDTO
}
