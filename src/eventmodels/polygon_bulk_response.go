package eventmodels

import "time"

type PolygonBulkResponse struct {
	Contracts []OptionContractV3
	TicksMap  map[ExpirationDate]map[OptionType]map[float64][]*OptionChainTickDTO
}

func (r *PolygonBulkResponse) mergeMaps(other map[ExpirationDate]map[OptionType]map[float64][]*OptionChainTickDTO) {
	if r.TicksMap == nil {
		r.TicksMap = make(map[ExpirationDate]map[OptionType]map[float64][]*OptionChainTickDTO)
	}

	for expDate, typeMap := range other {
		if _, exists := r.TicksMap[expDate]; !exists {
			r.TicksMap[expDate] = make(map[OptionType]map[float64][]*OptionChainTickDTO)
		}
		for optType, strikeMap := range typeMap {
			if _, exists := r.TicksMap[expDate][optType]; !exists {
				r.TicksMap[expDate][optType] = make(map[float64][]*OptionChainTickDTO)
			}
			for strike, ticks := range strikeMap {
				r.TicksMap[expDate][optType][strike] = ticks
			}
		}
	}
}

func (r *PolygonBulkResponse) Merge(other *PolygonBulkResponse) {
	if other == nil {
		return
	}

	r.Contracts = append(r.Contracts, other.Contracts...)
	r.mergeMaps(other.TicksMap)
}

func (r *PolygonBulkResponse) GetOptionContractsV3(loc *time.Location, spread float64) ([]OptionContractV3, map[ExpirationDate]map[OptionType]map[float64][]*OptionChainTickDTO, error) {
	return r.Contracts, r.TicksMap, nil
}
