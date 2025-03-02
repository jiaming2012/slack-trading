package services

import (
	"sync"
	"time"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterApiService struct {
	projectsDir       string
	polygonClient     models.IPolygonClient
	dbService         models.IDatabaseService
	liveRepositories  map[eventmodels.Instrument]map[time.Duration][]*models.CandleRepository
	liveAccountsMutex sync.Mutex
}

func (s *BacktesterApiService) GetDbService() models.IDatabaseService {
	return s.dbService
}

func NewBacktesterApiService(projectsDir string, polygonClient models.IPolygonClient, dbService models.IDatabaseService) *BacktesterApiService {
	return &BacktesterApiService{
		projectsDir:      projectsDir,
		polygonClient:    polygonClient,
		dbService:        dbService,
		liveRepositories: make(map[eventmodels.Instrument]map[time.Duration][]*models.CandleRepository),
	}
}
