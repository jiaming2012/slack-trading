package eventmodels

import (
	"encoding/json"
	"fmt"
	"time"
)

type OptionPriceGreeksDTO struct {
	Delta float64 `json:"delta"`
}

// separate file
type GreeksDTO struct {
	Delta     float64 `json:"delta"`
	Gamma     float64 `json:"gamma"`
	Theta     float64 `json:"theta"`
	Vega      float64 `json:"vega"`
	Rho       float64 `json:"rho"`
	Phi       float64 `json:"phi"`
	BidIv     float64 `json:"bid_iv"`
	MidIv     float64 `json:"mid_iv"`
	AskIv     float64 `json:"ask_iv"`
	SmvVol    float64 `json:"smv_vol"`
	UpdatedAt string  `json:"updated_at"`
}

type QuoteDTO struct {
	Symbol           string    `json:"symbol"`
	Description      string    `json:"description"`
	Exch             string    `json:"exch"`
	Type             string    `json:"type"`
	LastPrice        float64   `json:"last"`
	Change           float64   `json:"change"`
	Volume           int       `json:"volume"`
	Open             *float64  `json:"open"`
	High             *float64  `json:"high"`
	Low              *float64  `json:"low"`
	Close            *float64  `json:"close"`
	Bid              float64   `json:"bid"`
	Ask              float64   `json:"ask"`
	Underlying       string    `json:"underlying"`
	Strike           float64   `json:"strike"`
	Greeks           GreeksDTO `json:"greeks"`
	ChangePercentage float64   `json:"change_percentage"`
	AverageVolume    int       `json:"average_volume"`
	LastVolume       int       `json:"last_volume"`
	TradeDate        int64     `json:"trade_date"`
	Prevclose        float64   `json:"prevclose"`
	Week52High       float64   `json:"week_52_high"`
	Week52Low        float64   `json:"week_52_low"`
	Bidsize          int       `json:"bidsize"`
	Bidexch          *string   `json:"bidexch"`
	BidDate          int64     `json:"bid_date"`
	Asksize          int       `json:"asksize"`
	Askexch          string    `json:"askexch"`
	AskDate          int64     `json:"ask_date"`
	OpenInterest     int       `json:"open_interest"`
	ContractSize     int       `json:"contract_size"`
	ExpirationDate   string    `json:"expiration_date"`
	ExpirationType   string    `json:"expiration_type"`
	OptionType       string    `json:"option_type"`
	RootSymbol       string    `json:"root_symbol"`
}

type UnmatchedSymbolsDTO struct {
	Symbol []string `json:"symbol"`
}

type UnmatchedSymbolDTO struct {
	Symbol string `json:"symbol"`
}

type QuotesRawDTO struct {
	Quote            *json.RawMessage `json:"quote"`
	UnmatchedSymbols *json.RawMessage `json:"unmatched_symbols"`
}

type OptionQuotesDTO struct {
	Quotes QuotesRawDTO `json:"quotes"`
}

func (dto *OptionQuotesDTO) Parse() ([]QuoteDTO, UnmatchedSymbolsDTO, error) {
	var quotes []QuoteDTO
	if dto.Quotes.Quote != nil {
		// try to unmarshal as a list of quotes
		if quoteListErr := json.Unmarshal(*dto.Quotes.Quote, &quotes); quoteListErr != nil {

			// if it fails, try to unmarshal as a single quote
			var quote QuoteDTO
			if quoteSingleErr := json.Unmarshal(*dto.Quotes.Quote, &quote); quoteSingleErr != nil {
				return nil, UnmatchedSymbolsDTO{}, fmt.Errorf("Parse: Error decoding JSON: %v", quoteSingleErr)
			} else {
				quotes = append(quotes, quote)
			}
		}
	}

	var unmatchedSymbols UnmatchedSymbolsDTO
	if dto.Quotes.UnmatchedSymbols != nil {
		// try to unmarshal as a list of unmatched symbols
		if unmatchedSymbolsErr := json.Unmarshal(*dto.Quotes.UnmatchedSymbols, &unmatchedSymbols); unmatchedSymbolsErr != nil {

			// if it fails, try to unmarshal as a single unmatched symbol
			var unmatchedSymbol UnmatchedSymbolDTO
			if unmatchedSymbolErr := json.Unmarshal(*dto.Quotes.UnmatchedSymbols, &unmatchedSymbol); unmatchedSymbolErr != nil {
				return nil, UnmatchedSymbolsDTO{}, fmt.Errorf("Parse: Error decoding JSON: %v", unmatchedSymbolErr)
			} else {
				unmatchedSymbols.Symbol = append(unmatchedSymbols.Symbol, unmatchedSymbol.Symbol)
			}
		}
	}

	return quotes, unmatchedSymbols, nil
}

func (dto *OptionQuotesDTO) ToModel() (OptionQuoteMap, error) {
	optionQuotes := make(OptionQuoteMap)

	quotes, _, err := dto.Parse()
	if err != nil {
		return nil, fmt.Errorf("OptionQuotesDTO.ToModel: Error parsing dto: %w", err)
	}

	// if len(unmatchedSymbols.Symbol) > 0 {
	// 	return nil, fmt.Errorf("OptionQuotesDTO.ToModel: Unmatched symbols found: %v", unmatchedSymbols.Symbol)
	// }

	for _, item := range quotes {
		expirationDate, err := time.Parse("2006-01-02", item.ExpirationDate)
		if err != nil {
			return nil, fmt.Errorf("OptionPriceDTO:ToModel(): failed to parse expiration date: %w", err)
		}

		optionQuotes[item.Symbol] = OptionQuote{
			Symbol:         item.Symbol,
			LastPrice:      item.LastPrice,
			Underlying:     item.Underlying,
			Delta:          item.Greeks.Delta,
			ExpirationDate: expirationDate,
		}
	}

	return optionQuotes, nil
}
