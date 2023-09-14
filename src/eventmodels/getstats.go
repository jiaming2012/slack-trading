package eventmodels

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"net/http"
	"slack-trading/src/models"
)

type GetStatsRequest struct {
	RequestID   uuid.UUID `json:"requestID"`
	AccountName string    `json:"AccountName"`
}

func (r *GetStatsRequest) GetRequestID() uuid.UUID {
	return r.RequestID
}

func (r *GetStatsRequest) Validate(request *http.Request) error {
	vars := mux.Vars(request)
	accountName, found := vars["accountName"]
	if !found {
		return fmt.Errorf("GetStatsRequest.Validate: could not find account name in path parameters")
	}

	r.AccountName = accountName

	if r.AccountName == "" {
		return fmt.Errorf("GetStatsRequest.Validate: account name was not set")
	}

	return nil
}

type GetStatsResultItem struct {
	StrategyName string                     `json:"strategyName"`
	Stats        *models.TradeStats         `json:"stats"`
	OpenTrades   []*models.PriceLevelTrades `json:"openTrades"`
}

type GetStatsResult struct {
	RequestID  uuid.UUID             `json:"requestID"`
	Strategies []*GetStatsResultItem `json:"strategies"`
}

func (r *GetStatsResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
