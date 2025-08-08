package eventmodels

import "time"

type PolygonBulkResponse struct {
	Contracts []OptionContractV3
	TicksMap  map[ExpirationDate][]*OptionChainTickDTO
}

func (r *PolygonBulkResponse) mergeMaps(other map[ExpirationDate][]*OptionChainTickDTO) {
	for k, v := range other {
		if existing, found := r.TicksMap[k]; found {
			r.TicksMap[k] = append(existing, v...)
		} else {
			r.TicksMap[k] = v
		}
	}
}

func (r *PolygonBulkResponse) Merge(other *PolygonBulkResponse) {
	if other == nil {
		return
	}

	r.Contracts = append(r.Contracts, other.Contracts...)
	for k, v := range other.TicksMap {
		r.TicksMap[k] = append(r.TicksMap[k], v...)
	}

	r.mergeMaps(other.TicksMap)
}

func (r *PolygonBulkResponse) GetOptionContractsV3(loc *time.Location, spread float64) ([]OptionContractV3, map[ExpirationDate][]*OptionChainTickDTO, error) {
	return r.Contracts, r.TicksMap, nil
}
