package eventmodels

import (
	"fmt"
	"time"
)

type OptionContractDetailDTO struct {
	Date           string           `json:"date"`
	ContractSize   int              `json:"contract_size"`
	ExpirationType string           `json:"expiration_type"`
	Strikes        OptionStrikesDTO `json:"strikes"`
}

func (dto *OptionContractDetailDTO) convertToOptionContract(optionTypes []OptionType) ([]OptionContract, error) {
	expiration, err := time.Parse("2006-01-02", dto.Date)
	if err != nil {
		return nil, fmt.Errorf("convertToOptionContract: failed to parse expiration date: %w", err)
	}

	var contracts []OptionContract

	for _, optionType := range optionTypes {
		for _, strike := range dto.Strikes.Strike {
			contract := OptionContract{
				Expiration:     expiration,
				ContractSize:   dto.ContractSize,
				ExpirationType: dto.ExpirationType,
				Strike:         strike,
				OptionType:     optionType,
			}

			contracts = append(contracts, contract)
		}
	}

	return contracts, nil
}

type OptionStrikesDTO struct {
	Strike []float64 `json:"strike"`
}

type OptionExpirationsDTO struct {
	Values []OptionContractDetailDTO `json:"expiration"`
}

type OptionContractDTO struct {
	Expirations OptionExpirationsDTO `json:"expirations"`
}

func (dto *OptionContractDTO) ConvertToOptionContracts(optionTypes []OptionType) (map[time.Time][]OptionContract, error) {
	contracts := make(map[time.Time][]OptionContract)

	for _, contractDetailDTO := range dto.Expirations.Values {
		convertedContracts, err := contractDetailDTO.convertToOptionContract(optionTypes)
		if err != nil {
			return nil, fmt.Errorf("ConvertToOptionContracts: failed to convert expiration to contract: %w", err)
		}

		expiration, err := time.Parse("2006-01-02", contractDetailDTO.Date)
		if err != nil {
			return nil, fmt.Errorf("ConvertToOptionContracts: failed to parse expiration date: %w", err)
		}

		contracts[expiration] = convertedContracts
	}

	return contracts, nil
}