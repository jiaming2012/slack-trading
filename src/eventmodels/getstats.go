package eventmodels

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"net/http"
	"slack-trading/src/models"
	"time"
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
	StrategyName    string                      `json:"name"`
	Stats           *models.TradeStats          `json:"stats"`
	EntryConditions []*models.EntryConditionDTO `json:"entryConditions"`
	ExitConditions  []*models.ExitConditionDTO  `json:"exitConditions"`
	OpenTradeLevels []*models.TradeLevels       `json:"openTrades"`
	CreatedOn       time.Time                   `json:"createdOn"`
}

type GetStatsResult struct {
	RequestID  uuid.UUID             `json:"requestID"`
	Strategies []*GetStatsResultItem `json:"strategies"`
}

func (r *GetStatsResult) GetRequestID() uuid.UUID {
	return r.RequestID
}
