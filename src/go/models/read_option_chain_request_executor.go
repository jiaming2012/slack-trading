package models

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

type ReadOptionChainRequestExecutor struct {
	StockHistoricalPricesURL string
	OptionsByExpirationURL   string
	OptionChainURL           string
	StockURL                 string
	BearerToken              string
	GoEnv                    string
	OptionsDataFetcher       OptionsDataFetcher
}

func (s *ReadOptionChainRequestExecutor) formatOptionContractSpreads(expectedProfitSpreadMap map[string]ExpectedProfitItemSpread) (map[string][]*OptionSpreadContractDTO, error) {
	var callOptionsDTO []*OptionSpreadContractDTO
	var putOptionsDTO []*OptionSpreadContractDTO

	for _, spreadMapItem := range expectedProfitSpreadMap {
		// shortSpread, found := expectedProfitShortSpreadMap[description]
		// if !found {
		// 	return nil, fmt.Errorf("formatOptionContractSpreads: missing short spread for description: %s", description)
		// }

		spread := OptionSpreadContractDTO{
			Timestamp:               GetMinTime(spreadMapItem.LongOptionTimestamp, spreadMapItem.ShortOptionTimestamp),
			Description:             spreadMapItem.Description,
			DebitPaid:               spreadMapItem.DebitPaid,
			CreditReceived:          spreadMapItem.CreditReceived,
			LongOptionTimestamp:     spreadMapItem.LongOptionTimestamp,
			LongOptionSymbol:        OptionSymbol(spreadMapItem.LongOptionSymbol),
			LongOptionAvgFillPrice:  spreadMapItem.LongOptionAvgFillPrice,
			LongOptionExpiration:    spreadMapItem.LongOptionExpiration,
			LongOptionStrikePrice:   spreadMapItem.LongOptionStrikePrice,
			ShortOptionTimestamp:    spreadMapItem.ShortOptionTimestamp,
			ShortOptionSymbol:       OptionSymbol(spreadMapItem.ShortOptionSymbol),
			ShortOptionExpiration:   spreadMapItem.ShortOptionExpiration,
			ShortOptionAvgFillPrice: spreadMapItem.ShortOptionAvgFillPrice,
			ShortOptionStrikePrice:  spreadMapItem.ShortOptionStrikePrice,
		}

		spread.Stats = OptionStats{
			ExpectedProfitLong:  0,
			ExpectedProfitShort: spreadMapItem.ExpectedProfit,
		}

		if spreadMapItem.Type == OptionTypeCallSpread {
			spread.Type = OptionTypeCallSpread
			callOptionsDTO = append(callOptionsDTO, &spread)
		} else if spreadMapItem.Type == OptionTypePutSpread {
			spread.Type = OptionTypePutSpread
			putOptionsDTO = append(putOptionsDTO, &spread)
		} else {
			return nil, fmt.Errorf("formatOptionContractSpreads: invalid spread type: %s", spreadMapItem.Type)
		}
	}

	sort.Slice(callOptionsDTO, func(i, j int) bool {
		return math.Max(callOptionsDTO[i].Stats.ExpectedProfitLong, callOptionsDTO[i].Stats.ExpectedProfitShort) > math.Max(callOptionsDTO[j].Stats.ExpectedProfitLong, callOptionsDTO[j].Stats.ExpectedProfitShort)
	})

	sort.Slice(putOptionsDTO, func(i, j int) bool {
		return math.Max(putOptionsDTO[i].Stats.ExpectedProfitLong, putOptionsDTO[i].Stats.ExpectedProfitShort) > math.Max(putOptionsDTO[j].Stats.ExpectedProfitLong, putOptionsDTO[j].Stats.ExpectedProfitShort)
	})

	return map[string][]*OptionSpreadContractDTO{
		"calls": callOptionsDTO,
		"puts":  putOptionsDTO,
	}, nil
}

func (s *ReadOptionChainRequestExecutor) formatOptionContracts(options []OptionContractV3, expectedProfitLongMap map[string]ExpectedProfitItem, expectedProfitShortMap map[string]ExpectedProfitItem) []*OptionContractV3DTO {
	now := time.Now()
	var optionsDTO []*OptionContractV3DTO
	for _, option := range options {
		dto := option.ToDTO(now)

		if profitLong, found := expectedProfitLongMap[option.Description]; found {
			if profitLong.DebitPaid == nil {
				continue
			}

			dto.Stats.ExpectedProfitLong = profitLong.ExpectedProfit
		}

		if profitShort, found := expectedProfitShortMap[option.Description]; found {
			if profitShort.CreditReceived == nil {
				continue
			}

			dto.Stats.ExpectedProfitShort = profitShort.ExpectedProfit
		}

		optionsDTO = append(optionsDTO, dto)
	}

	sort.Slice(optionsDTO, func(i, j int) bool {
		return math.Max(optionsDTO[i].Stats.ExpectedProfitLong, optionsDTO[i].Stats.ExpectedProfitShort) > math.Max(optionsDTO[j].Stats.ExpectedProfitLong, optionsDTO[j].Stats.ExpectedProfitShort)
	})

	return optionsDTO
}


func (s *ReadOptionChainRequestExecutor) ServeWithParams(ctx context.Context, req *ReadOptionChainRequest, inputData FetchOptionChainDataInput, bFindSpreads bool, projectsDir string, now time.Time, resultCh chan map[string]interface{}, errorCh chan error) {
	tracer := otel.Tracer("ReadOptionChainRequestExecutor")
	ctx, span := tracer.Start(ctx, "ReadOptionChainRequestExecutor.ServeWithParams")
	defer span.End()

	result := map[string]interface{}{
		"stock": map[string]interface{}{
			"timestamp": inputData.StockTickItemDTO.Timestamp,
			"bid":       inputData.StockTickItemDTO.Bid,
			"ask":       inputData.StockTickItemDTO.Ask,
		},
	}

	startPeriodStr := req.EV.StartsAt.Format("2006-01-02T00:00:00")
	endPeriodStr := req.EV.EndsAt.Format("2006-01-02T00:00:00")

	log.Infof("Calculating EV from startPeriod: %v to endPeriod: %v", startPeriodStr, endPeriodStr)

	_, expectedProfitShortSpreads, err := s.OptionsDataFetcher.FetchEVSpreads(
		ctx,
		projectsDir,
		req.EV.Signal,
		bFindSpreads,
		req.EV.StartsAt,
		req.EV.EndsAt,
		req.Symbol,
		s.GoEnv,
		inputData.OptionContracts,
		inputData.StockTickItemDTO,
		now)

	if err != nil {
		errorCh <- err
		return
	}

	output, err := s.formatOptionContractSpreads(expectedProfitShortSpreads)
	if err != nil {
		errorCh <- err
		return
	}

	result["options"] = output

	resultCh <- result
}
