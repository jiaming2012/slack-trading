package eventmodels

import "time"

type PolygonBulkResponse struct {
	Contracts []OptionContractV3
	TicksMap  map[ExpirationDate][]*OptionChainTickDTO
}

func (r *PolygonBulkResponse) GetOptionContractsV3(loc *time.Location, spread float64) ([]OptionContractV3, map[ExpirationDate][]*OptionChainTickDTO, error) {
	return r.Contracts, r.TicksMap, nil
}
