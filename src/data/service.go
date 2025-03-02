package data

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func (s *DatabaseService) GetPlaygroundByClientId(clientId string) models.IPlayground {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, playground := range s.playgrounds {
		cId := playground.GetClientId()
		if cId != nil && *cId == clientId {
			return playground
		}
	}

	return nil
}

func (s *DatabaseService) GetPlayground(playgroundID uuid.UUID) (models.IPlayground, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, ok := s.playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	return playground, nil
}

func (s *DatabaseService) GetPlaygrounds() []models.IPlayground {
	s.mu.Lock()
	defer s.mu.Unlock()

	var slice []models.IPlayground
	for _, playground := range s.playgrounds {
		slice = append(slice, playground)
	}

	return slice
}

func (s *DatabaseService) DeletePlayground(playgroundID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.playgrounds[playgroundID]
	if !ok {
		return eventmodels.NewWebError(404, "playground not found", nil)
	}

	delete(s.playgrounds, playgroundID)

	return nil
}

func (s *DatabaseService) FetchLiveAccount(source *models.CreateAccountRequestSource) (models.ILiveAccount, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if source == nil {
		return nil, false, fmt.Errorf("FetchLiveAccount: source is nil")
	}

	if source.Broker == "" {
		return nil, false, fmt.Errorf("FetchLiveAccount: broker is empty")
	}

	if source.AccountID == "" {
		return nil, false, fmt.Errorf("FetchLiveAccount: account id is empty")
	}

	if source.AccountType == "" {
		return nil, false, fmt.Errorf("FetchLiveAccount: account type is empty")
	}

	account, ok := s.liveAccounts[*source]
	if !ok {
		return nil, false, nil
	}

	return account, true, nil
}
