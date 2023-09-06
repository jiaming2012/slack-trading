package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/models"
	"sync"
)

type AccountWorker struct {
	wg       *sync.WaitGroup
	accounts []*models.Account
}

func (r *AccountWorker) getAccounts() []models.Account {
	var accounts []models.Account

	for _, acc := range r.accounts {
		accounts = append(accounts, *acc)
	}

	return accounts
}

func (r *AccountWorker) addAccountRequestHandler(request eventmodels.AddAccountRequestEvent) {
	var priceLevels models.PriceLevels

	for _, input := range request.PriceLevelsInput {
		priceLevels.Values = append(priceLevels.Values, &models.PriceLevel{
			Price:             input[0],
			NoOfTrades:        int(input[1]),
			AllocationPercent: input[2],
		})
	}

	if err := priceLevels.Validate(); err != nil {
		pubsub.PublishError("AccountWorker.addAccountHandler", err)
		return
	}

	account, err := models.NewAccount(request.Name, request.Balance, request.MaxLossPercentage, priceLevels)
	if err != nil {
		pubsub.PublishError("AccountWorker.addAccountHandler", err)
		return
	}

	r.accounts = append(r.accounts, account)

	pubsub.Publish("AccountWorker.addAccountHandler", pubsub.AddAccountResponseEvent, eventmodels.AddAccountResponseEvent{
		Account: *account,
	})
}

func (r *AccountWorker) getAccountsRequestHandler(request eventmodels.GetAccountsRequestEvent) {
	log.Debugf("AccountWorker.getAccountsRequestHandler")

	pubsub.Publish("AccountWorker", pubsub.GetAccountsResponseEvent, eventmodels.GetAccountsResponseEvent{
		Accounts: r.getAccounts(),
	})
}

func (r *AccountWorker) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("AccountWorker", pubsub.AddAccountRequestEvent, r.addAccountRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.GetAccountsRequestEvent, r.getAccountsRequestHandler)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping AccountWorker consumer")
				return
			}
		}
	}()
}

func NewAccountWorkerClientFromFixtures(wg *sync.WaitGroup, accounts []*models.Account) *AccountWorker {
	return &AccountWorker{
		wg:       wg,
		accounts: accounts,
	}
}

func NewAccountWorkerClient(wg *sync.WaitGroup) *AccountWorker {
	return &AccountWorker{
		wg:       wg,
		accounts: make([]*models.Account, 0),
	}
}
