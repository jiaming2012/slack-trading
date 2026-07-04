package data

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

// liveAccountStore owns the live-account cache, the broker map, and the live
// candle repositories.
//
// Locking: liveAccounts and brokerMap are guarded by the service-wide mutex
// held by the DatabaseService public methods (unchanged from before the split).
// liveRepositories keeps its own dedicated mutex (formerly
// DatabaseService.liveAccountsMutex), moved here verbatim — same granularity,
// same three methods use it.
type liveAccountStore struct {
	db                *gorm.DB
	liveAccounts      map[models.CreateAccountRequestSource]models.ILiveAccount
	liveRepositories  map[eventmodels.Instrument]map[time.Duration][]*models.CandleRepository
	brokerMap         map[models.CreateAccountRequestSource]models.IBroker
	liveAccountsMutex sync.Mutex
}

func newLiveAccountStore(db *gorm.DB) *liveAccountStore {
	return &liveAccountStore{
		db:               db,
		liveAccounts:     make(map[models.CreateAccountRequestSource]models.ILiveAccount),
		liveRepositories: make(map[eventmodels.Instrument]map[time.Duration][]*models.CandleRepository),
	}
}

func (st *liveAccountStore) getLiveAccount(source models.CreateAccountRequestSource) (models.ILiveAccount, error) {
	liveAccount, found := st.liveAccounts[source]
	if !found {
		return nil, fmt.Errorf("failed to find live account: %v", source)
	}

	return liveAccount, nil
}

func (st *liveAccountStore) fetchLiveAccount(source *models.CreateAccountRequestSource) (models.ILiveAccount, bool, error) {
	if source == nil {
		return nil, false, fmt.Errorf("FetchLiveAccount: source is nil")
	}

	if source.Broker == "" {
		return nil, false, fmt.Errorf("FetchLiveAccount: broker is empty")
	}

	if source.AccountID == "" {
		return nil, false, fmt.Errorf("FetchLiveAccount: account id is empty")
	}

	if source.LiveAccountType == "" {
		return nil, false, fmt.Errorf("FetchLiveAccount: account type is empty")
	}

	account, ok := st.liveAccounts[*source]
	if !ok {
		return nil, false, nil
	}

	return account, true, nil
}

func (st *liveAccountStore) getMockBroker(broker string) (models.IBroker, error) {
	for b, val := range st.brokerMap {
		if b.LiveAccountType == models.LiveAccountTypeMock {
			if b.Broker == broker {
				return val, nil
			}
		}
	}

	return &models.MockBroker{}, fmt.Errorf("failed to find mock broker: %s", broker)
}

// populateLiveAccount wires broker and database dependencies into a live
// account. owner is the enclosing DatabaseService (accounts hold a reference
// back to the full IDatabaseService).
func (st *liveAccountStore) populateLiveAccount(a *models.LiveAccount, owner models.IDatabaseService) error {
	if a.BrokerName != "tradier" {
		return fmt.Errorf("unsupported broker: %s", a.BrokerName)
	}

	if st.brokerMap == nil {
		return fmt.Errorf("must call LoadLiveAccounts before calling PopulateLiveAccount")
	}

	source := a.GetSource()
	broker, found := st.brokerMap[source]

	if !found {
		return fmt.Errorf("loadLiveAccounts: failed to find broker: %v", a.BrokerName)
	}

	a.SetBroker(broker)
	a.SetDatabase(owner)

	return nil
}

func (st *liveAccountStore) loadLiveAccounts(brokerMap map[models.CreateAccountRequestSource]models.IBroker, owner models.IDatabaseService) error {
	var liveAccountsRecords []*models.LiveAccount

	st.brokerMap = brokerMap

	if err := st.db.Find(&liveAccountsRecords).Error; err != nil {
		return fmt.Errorf("loadLiveAccounts: failed to load live accounts: %w", err)
	}

	for _, a := range liveAccountsRecords {
		source := a.GetSource()

		broker, found := brokerMap[source]
		if !found {
			return fmt.Errorf("loadLiveAccounts: failed to find broker: %v", a.BrokerName)
		}

		a.SetBroker(broker)
		a.SetDatabase(owner)

		st.liveAccounts[source] = a
	}

	for source, broker := range brokerMap {
		if _, found := st.liveAccounts[source]; !found {
			a, err := models.NewLiveAccount(broker, owner)
			if err != nil {
				return fmt.Errorf("failed to create live account: %w", err)
			}

			a.SetBroker(broker)
			a.SetDatabase(owner)

			if err := st.db.Save(a).Error; err != nil {
				return fmt.Errorf("failed to save live account: %w", err)
			}

			st.liveAccounts[source] = a
		}
	}

	log.Info("loaded all live accounts")

	return nil
}

func (st *liveAccountStore) fetchAllLiveRepositories() (repositories []*models.CandleRepository, releaseLockFn func(), err error) {
	st.liveAccountsMutex.Lock()
	defer st.liveAccountsMutex.Unlock()

	repositories = []*models.CandleRepository{}
	for _, symbolRepo := range st.liveRepositories {
		for _, periodRepos := range symbolRepo {
			repositories = append(repositories, periodRepos...)
		}
	}

	return repositories, func() {
		st.liveAccountsMutex.Unlock()
	}, nil
}

func (st *liveAccountStore) removeLiveRepository(repo *models.CandleRepository) error {
	st.liveAccountsMutex.Lock()
	defer st.liveAccountsMutex.Unlock()

	symbolRepo, ok := st.liveRepositories[repo.GetSymbol()]
	if !ok {
		return fmt.Errorf("DeleteLiveRepository: symbol %s not found", repo.GetSymbol())
	}

	periodRepos, ok := symbolRepo[repo.GetPeriod()]
	if !ok {
		return fmt.Errorf("DeleteLiveRepository: period %s not found", repo.GetPeriod())
	}

	foundRepo := false
	for i, r := range periodRepos {
		if r == repo {
			periodRepos = append(periodRepos[:i], periodRepos[i+1:]...)
			foundRepo = true
			break
		}
	}

	if !foundRepo {
		return fmt.Errorf("DeleteLiveRepository: repository not found")
	}

	symbolRepo[repo.GetPeriod()] = periodRepos
	st.liveRepositories[repo.GetSymbol()] = symbolRepo

	return nil
}

func (st *liveAccountStore) saveLiveRepository(repo *models.CandleRepository) error {
	st.liveAccountsMutex.Lock()
	defer st.liveAccountsMutex.Unlock()

	symbolRepo, ok := st.liveRepositories[repo.GetSymbol()]
	if !ok {
		symbolRepo = map[time.Duration][]*models.CandleRepository{}
	}

	periodRepos, ok := symbolRepo[repo.GetPeriod()]
	if !ok {
		periodRepos = []*models.CandleRepository{}
	}

	// append the repo to the periodRepos
	periodRepos = append(periodRepos, repo)
	symbolRepo[repo.GetPeriod()] = periodRepos
	st.liveRepositories[repo.GetSymbol()] = symbolRepo

	return nil
}

// --- DatabaseService public surface: thin delegations to liveAccountStore ---

func (s *DatabaseService) GetLiveAccount(source models.CreateAccountRequestSource) (models.ILiveAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.liveAccountStore.getLiveAccount(source)
}

func (s *DatabaseService) FetchLiveAccount(source *models.CreateAccountRequestSource) (models.ILiveAccount, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.liveAccountStore.fetchLiveAccount(source)
}

func (s *DatabaseService) GetMockBroker(broker string) (models.IBroker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.liveAccountStore.getMockBroker(broker)
}

func (s *DatabaseService) PopulateLiveAccount(a *models.LiveAccount) error {
	return s.liveAccountStore.populateLiveAccount(a, s)
}

func (s *DatabaseService) LoadLiveAccounts(brokerMap map[models.CreateAccountRequestSource]models.IBroker) error {
	return s.liveAccountStore.loadLiveAccounts(brokerMap, s)
}

func (s *DatabaseService) FetchAllLiveRepositories() (repositories []*models.CandleRepository, releaseLockFn func(), err error) {
	return s.liveAccountStore.fetchAllLiveRepositories()
}

func (s *DatabaseService) RemoveLiveRepository(repo *models.CandleRepository) error {
	return s.liveAccountStore.removeLiveRepository(repo)
}

func (s *DatabaseService) SaveLiveRepository(repo *models.CandleRepository) error {
	return s.liveAccountStore.saveLiveRepository(repo)
}

func (s *DatabaseService) FetchBalances(url string, token string) (eventmodels.FetchTradierBalancesResponseDTO, error) {
	return eventmodels.FetchTradierBalancesResponseDTO{}, nil
}
