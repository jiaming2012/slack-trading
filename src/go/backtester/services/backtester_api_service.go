package services

// 	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
// 	"github.com/jiaming2012/slack-trading/src/go/models"
// )

// type BacktesterApiService struct {
// 	projectsDir       string
// 	polygonClient     backtester_models.IPolygonClient
// 	dbService         backtester_models.IDatabaseService
// 	liveRepositories  map[models.Instrument]map[time.Duration][]*backtester_models.CandleRepository
// 	liveAccountsMutex sync.Mutex
// }

// func (s *BacktesterApiService) GetDbService() backtester_models.IDatabaseService {
// 	return s.dbService
// }

// func NewBacktesterApiService(projectsDir string, polygonClient backtester_models.IPolygonClient, dbService backtester_models.IDatabaseService) *BacktesterApiService {
// 	return &BacktesterApiService{
// 		projectsDir:      projectsDir,
// 		polygonClient:    polygonClient,
// 		dbService:        dbService,
// 		liveRepositories: make(map[models.Instrument]map[time.Duration][]*backtester_models.CandleRepository),
// 	}
// }
