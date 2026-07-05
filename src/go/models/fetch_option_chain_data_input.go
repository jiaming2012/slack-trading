package models

type FetchOptionChainDataInput struct {
	OptionContracts  []OptionContractV3
	StockTickItemDTO *StockTickItemDTO
}
