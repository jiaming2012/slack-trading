package eventmodels

import "time"

type TrackerStart struct {
	ID                TrackerID          `json:"id"`
	Timestamp         time.Time          `json:"timestamp"`
	Reason            string             `json:"reason"`
	StockSymbol       string             `json:"stockSymbol"`
	OptionContractIDs []OptionContractID `json:"optionContractIDs"`
}
