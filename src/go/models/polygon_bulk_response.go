package models

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

// DeepCopy returns a new PolygonBulkResponse with independent copies of the
// Contracts slice and TicksMap so that Merge operations don't mutate cached values.
func (r *PolygonBulkResponse) DeepCopy() *PolygonBulkResponse {
	if r == nil {
		return nil
	}

	// Copy contracts slice
	contracts := make([]OptionContractV3, len(r.Contracts))
	copy(contracts, r.Contracts)

	// Copy ticks map (shallow copy of tick pointers is fine — we only need
	// structural independence so that Merge doesn't modify the cached map)
	ticksMap := make(map[ExpirationDate]map[OptionType]map[float64][]*OptionChainTickDTO, len(r.TicksMap))
	for expDate, typeMap := range r.TicksMap {
		newTypeMap := make(map[OptionType]map[float64][]*OptionChainTickDTO, len(typeMap))
		for optType, strikeMap := range typeMap {
			newStrikeMap := make(map[float64][]*OptionChainTickDTO, len(strikeMap))
			for strike, ticks := range strikeMap {
				ticksCopy := make([]*OptionChainTickDTO, len(ticks))
				copy(ticksCopy, ticks)
				newStrikeMap[strike] = ticksCopy
			}
			newTypeMap[optType] = newStrikeMap
		}
		ticksMap[expDate] = newTypeMap
	}

	return &PolygonBulkResponse{
		Contracts: contracts,
		TicksMap:  ticksMap,
	}
}
