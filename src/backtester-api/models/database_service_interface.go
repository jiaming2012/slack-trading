package models

import (
	"github.com/google/uuid"
)

type IDatabaseService interface {
	SaveOrderRecord(playgroundId uuid.UUID, order *BacktesterOrder, newBalance *float64, liveAccountType LiveAccountType) (*OrderRecord, error)
	LoadPlaygrounds(apiService IBacktesterApiService) error
	SavePlaygroundSession(playground IPlayground) (*PlaygroundSession, error)
	SaveLiveAccount(source *CreateAccountRequestSource, liveAccount ILiveAccount) error
	UpdatePlaygroundSession(playgroundSession *PlaygroundSession) error
	FetchLiveAccount(source *CreateAccountRequestSource) (ILiveAccount, bool, error)
	FetchPlayground(playgroundId uuid.UUID) (IPlayground, error)
	GetPlaygrounds() []IPlayground
	GetPlaygroundByClientId(clientId string) IPlayground
	GetPlayground(playgroundID uuid.UUID) (IPlayground, error)
	DeletePlayground(playgroundID uuid.UUID) error
	SaveInMemoryPlayground(p IPlayground) error
	FindOrder(playgroundId uuid.UUID, id uint) (IPlayground, *BacktesterOrder, error)
	FetchPendingOrders(accountType LiveAccountType) ([]*OrderRecord, error)
}
