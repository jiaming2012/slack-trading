package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type PopulatePlaygroundRequest struct {
	ID                  *uuid.UUID                            `json:"playground_id"`
	ClientID            *string                               `json:"client_id"`
	Env                 PlaygroundEnvironment                 `json:"environment"`
	Account             CreateAccountRequest                  `json:"account"`
	InitialBalance      float64                               `json:"starting_balance"`
	Clock               CreateClockRequest                    `json:"clock"`
	Repositories        []eventmodels.CreateRepositoryRequest `json:"repositories"`
	BackfillOrders      []*OrderRecord                        `json:"orders"`
	EquityPlotRecords   []*eventmodels.EquityPlot             `json:"equity_plot_records"`
	CreatedAt           time.Time                             `json:"created_at"`
	Tags                []string                              `json:"tags"`
	Calendar            *eventmodels.MarketCalendar           `json:"-"`
	SaveToDB            bool                                  `json:"-"`
	LiveAccount         ILiveAccount                          `json:"-"`
	ReconcilePlayground IReconcilePlayground                  `json:"-"`
}
