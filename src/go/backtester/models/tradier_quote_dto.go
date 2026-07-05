package models

type TradierQuoteDTO struct {
	Symbol           string  `json:"symbol"`
	Description      string  `json:"description"`
	Exch             string  `json:"exch"`
	Type             string  `json:"type"`
	Last             float64 `json:"last"`
	Change           float64 `json:"change"`
	Volume           int     `json:"volume"`
	Open             float64 `json:"open"`
	High             float64 `json:"high"`
	Low              float64 `json:"low"`
	Close            float64 `json:"close"`
	Bid              float64 `json:"bid"`
	Ask              float64 `json:"ask"`
	ChangePercentage float64 `json:"change_percentage"`
	AverageVolume    int     `json:"average_volume"`
	LastVolume       int     `json:"last_volume"`
	TradeDate        int64   `json:"trade_date"`
	PrevClose        float64 `json:"prevclose"`
	Week52High       float64 `json:"week_52_high"`
	Week52Low        float64 `json:"week_52_low"`
	BidSize          int     `json:"bidsize"`
	BidExch          string  `json:"bidexch"`
	BidDate          int64   `json:"bid_date"`
	AskSize          int     `json:"asksize"`
	AskExch          string  `json:"askexch"`
	AskDate          int64   `json:"ask_date"`
	RootSymbols      string  `json:"root_symbols"`
}