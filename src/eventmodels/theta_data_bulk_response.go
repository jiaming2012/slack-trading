package eventmodels

import (
	"fmt"
	"strconv"
	"time"
)

type ThetaDataBulkResponse struct {
	Header   ThetaDataResponseHeader `json:"header"`
	Response []struct {
		Ticks    [][]interface{}         `json:"ticks"`
		Contract ThetaDataOptionContract `json:"contract"`
	} `json:"response"`
}

func (r *ThetaDataBulkResponse) StoreHeaderIndex(headerName string, headerIndexMap map[string]int) error {
	for i, v := range r.Header.Format {
		if v == headerName {
			headerIndexMap[v] = i
			return nil
		}
	}

	return fmt.Errorf("ThetaDataBulkResponse: StoreHeaderIndex: unable to find header %v", headerName)
}

func (r *ThetaDataBulkResponse) GetOptionContractsV3(loc *time.Location, spread float64) ([]OptionContractV3, map[ExpirationDate][]*OptionChainTickDTO, error) {
	contracts := make([]OptionContractV3, 0)
	optionChainTickMap := make(map[ExpirationDate][]*OptionChainTickDTO)
	contractSize := 100

	dtos, err := r.ToBulkHistOptionOhlcDTO()
	if err != nil {
		return nil, nil, fmt.Errorf("ThetaDataBulkResponse: GetOptionContractsV3: failed to convert to bulk hist option ohlc dto: %w", err)
	}

	for _, dto := range dtos {
		var optionType OptionType
		if dto.Contract.Right == ThetaDataOptionTypeCall {
			optionType = OptionTypeCall
		} else if dto.Contract.Right == ThetaDataOptionTypePut {
			optionType = OptionTypePut
		} else {
			return nil, nil, fmt.Errorf("ThetaDataBulkResponse: GetOptionContractsV3: invalid option type: %v", dto.Contract.Right)
		}

		expirationStr := strconv.Itoa(dto.Contract.Expiration)

		expiration, err := time.Parse("20060102", expirationStr)
		if err != nil {
			return nil, nil, fmt.Errorf("ThetaDataBulkResponse: GetOptionContractsV3: failed to parse expiration: %w, using %v", err, expirationStr)
		}

		components := OptionSymbolComponents{
			Underlying:  string(dto.Contract.Root),
			Expiration:  expiration,
			StrikePrice: dto.Contract.Strike,
			OptionType:  dto.Contract.Right,
		}

		ticker, err := NewOptionSymbol(components)

		if err != nil {
			return nil, nil, fmt.Errorf("ThetaDataBulkResponse: GetOptionContractsV3: failed to create option ticker: %w", err)
		}

		contracts = append(contracts, OptionContractV3{
			Symbol:           ticker,
			UnderlyingSymbol: StockSymbol(dto.Contract.Root),
			Strike:           dto.Contract.Strike,
			OptionType:       optionType,
			Expiration:       expiration,
			ExpirationDate:   ExpirationDate(expiration.Format("2006-01-02")),
			ContractSize:     contractSize,
		})

		//--- chain tick map
		expStr := expiration.Format("2006-01-02")
		if _, ok := optionChainTickMap[ExpirationDate(expStr)]; !ok {
			optionChainTickMap[ExpirationDate(expStr)] = make([]*OptionChainTickDTO, 0)
		}

		optionDescription, err := ticker.Description()
		if err != nil {
			return nil, nil, fmt.Errorf("ThetaDataBulkResponse: GetOptionContractsV3: failed to get option description: %w", err)
		}

		for _, candleDTO := range dto.Candles {
			candle, err := candleDTO.ToHistOptionOhlc(loc)
			if err != nil {
				return nil, nil, fmt.Errorf("ThetaDataBulkResponse: GetOptionContractsV3: failed to convert to hist option ohlc: %w", err)
			}

			optionChainTickMap[ExpirationDate(expStr)] = append(optionChainTickMap[ExpirationDate(expStr)], &OptionChainTickDTO{
				Symbol:       string(ticker),
				Description:  optionDescription,
				Bid:          candle.Open,
				Ask:          candle.Open * (1 + spread),
				OptionType:   string(optionType),
				Strike:       dto.Contract.Strike,
				ContractSize: contractSize,
			})
		}

	}

	return contracts, optionChainTickMap, nil
}

func (r *ThetaDataBulkResponse) ToBulkHistOptionOhlcDTO() ([]*BulkHistOptionOhlcDTO, error) {
	headers := make(map[string]int)

	if err := r.StoreHeaderIndex("ms_of_day", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("open", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("high", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("low", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("close", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("volume", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	if err := r.StoreHeaderIndex("date", headers); err != nil {
		return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: %w", err)
	}

	out := make([]*BulkHistOptionOhlcDTO, 0)

	for _, dto := range r.Response {
		ticks, err := getTicks(headers, dto.Ticks)
		if err != nil {
			return nil, fmt.Errorf("ThetaDataHistOptionOHLCResponse: failed to convert ticks: %w", err)
		}

		out = append(out, &BulkHistOptionOhlcDTO{
			Contract: dto.Contract,
			Candles:  ticks,
		})
	}

	return out, nil
}
