package data

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func (s *DatabaseService) SaveLiveAccount(source *models.CreateAccountRequestSource, liveAccount models.ILiveAccount) error {
	_source := *source

	_, found := s.liveAccounts[_source]
	if found {
		return fmt.Errorf("saveLiveAccount: live account already exists with source: %v", _source)
	}

	s.liveAccounts[_source] = liveAccount

	return nil
}

func fetchOrderIdFromDbByExternalOrderId(playgroundId uuid.UUID, externalOrderID uint) (uint, bool) {
	var orderRecord models.OrderRecord

	if result := db.First(&orderRecord, "playground_id = ? AND external_id = ?", playgroundId, externalOrderID); result.Error != nil {
		return 0, false
	}

	return orderRecord.ID, true
}

func saveOrderRecordsTx(tx *gorm.DB, playgroundId uuid.UUID, orders []*models.BacktesterOrder, liveAccountType *models.LiveAccountType) ([]*models.OrderRecord, error) {
	var allOrderRecords []*models.OrderRecord
	var updateOrderRequests []*models.UpdateOrderRecordRequest

	for _, order := range orders {
		var err error

		oRec, updateOrderReq, err := order.UpdateOrderRecord(tx, playgroundId, liveAccountType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert order to order record: %w", err)
		}

		updateOrderRequests = append(updateOrderRequests, updateOrderReq...)

		oID, found := fetchOrderIdFromDbByExternalOrderId(playgroundId, oRec.ExternalOrderID)
		if found {
			oRec.ID = oID
		}

		if err = tx.Save(&oRec).Error; err != nil {
			return nil, fmt.Errorf("failed to save order records: %w", err)
		}

		allOrderRecords = append(allOrderRecords, oRec)
	}

	// wait for all orders to be saved before updating the closes
	for _, updateReq := range updateOrderRequests {
		if updateReq == nil {
			continue
		}

		switch updateReq.Field {
		case "closes":
			var closes []*models.OrderRecord
			for _, order := range updateReq.Closes {
				orderRec, err := order.FetchOrderRecordFromDB(tx, *updateReq.PlaygroundId)
				if err != nil {
					return nil, fmt.Errorf("updateOrderRequests: failed to fetch close order record from db: %w", err)
				}

				closes = append(closes, orderRec)
			}

			updateReq.OrderRecord.Closes = closes
			if err := tx.Save(updateReq.OrderRecord).Error; err != nil {
				return nil, fmt.Errorf("updateOrderRequests: failed to update order record (closes): %w", err)
			}

		case "reconciles":
			updateReq.OrderRecord.Reconciles = updateReq.Reconciles
			if err := tx.Save(updateReq.OrderRecord).Error; err != nil {
				return nil, fmt.Errorf("updateOrderRequests: failed to update order record (reconciled_by): %w", err)
			}

		case "closed_by":
			updateReq.OrderRecord.ClosedBy = updateReq.ClosedBy
			if err := tx.Save(updateReq.OrderRecord).Error; err != nil {
				return nil, fmt.Errorf("updateOrderRequests: failed to update order record (close_by): %w", err)
			}

		default:
			return nil, fmt.Errorf("updateOrderRequests: field %s not implemented", updateReq.Field)
		}
	}

	return allOrderRecords, nil
}

func saveBalance(tx *gorm.DB, playgroundId uuid.UUID, balance float64) error {
	if result := tx.Model(&models.PlaygroundSession{}).Where("id = ?", playgroundId).Update("balance", balance); result.Error != nil {
		return fmt.Errorf("saveBalance: failed to save balance: %w", result.Error)
	}

	return nil
}

func findOrderRec(id uint, orders []*models.OrderRecord) (*models.OrderRecord, error) {
	for _, order := range orders {
		if order.ID == id {
			return order, nil
		}
	}

	return nil, fmt.Errorf("findOrderRec: failed to find order record: %d", id)
}

func DeletePlaygroundSession(playground models.IPlayground) error {
	session := &models.PlaygroundSession{
		ID: playground.GetId(),
	}

	if err := db.Delete(&session).Error; err != nil {
		return fmt.Errorf("deletePlayground: failed to delete playground: %w", err)
	}

	return nil
}

func SavePlayground(playground models.IPlayground) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		var txErr error

		if _, txErr = savePlaygroundSessionTx(tx, playground); txErr != nil {
			return fmt.Errorf("failed to save playground session: %w", txErr)
		}

		playgroundId := playground.GetId()
		meta := playground.GetMeta()
		if meta == nil {
			return errors.New("savePlayground: missing playground meta")
		}

		accountType := meta.LiveAccountType
		if _, ok := playground.(*models.Playground); ok {
			accountType = models.LiveAccountTypeSimulator
		}

		if _, txErr = saveOrderRecordsTx(tx, playgroundId, playground.GetOrders(), &accountType); txErr != nil {
			return fmt.Errorf("failed to save order records: %w", txErr)
		}

		if txErr = saveEquityPlotRecords(tx, playgroundId, playground.GetEquityPlot()); txErr != nil {
			return fmt.Errorf("failed to save equity plot records: %w", txErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("savePlayground: failed to save playground: %w", err)
	}

	return nil
}

func saveEquityPlotRecords(tx *gorm.DB, playgroundId uuid.UUID, records []*eventmodels.EquityPlot) error {
	var equityPlotRecords []*models.EquityPlotRecord

	for _, record := range records {
		equityPlotRecords = append(equityPlotRecords, &models.EquityPlotRecord{
			PlaygroundID: playgroundId,
			Timestamp:    record.Timestamp,
			Equity:       record.Value,
		})
	}

	if err := tx.CreateInBatches(equityPlotRecords, 100).Error; err != nil {
		return fmt.Errorf("saveEquityPlotRecords: failed to save equity plot records: %w", err)
	}

	return nil
}

func SaveEquityPlotRecord(playgroundId uuid.UUID, timestamp time.Time, equity float64) error {
	rec := &models.EquityPlotRecord{
		PlaygroundID: playgroundId,
		Timestamp:    timestamp,
		Equity:       equity,
	}

	if err := db.Create(rec).Error; err != nil {
		return fmt.Errorf("SaveEquityPlotRecord: failed to save equity plot record: %w", err)
	}

	return nil
}

func savePlaygroundSessionTx(tx *gorm.DB, playground models.IPlayground) (*models.PlaygroundSession, error) {
	meta := playground.GetMeta()

	if err := meta.Validate(); err != nil {
		return nil, fmt.Errorf("savePlaygroundSession: invalid playground meta: %w", err)
	}

	repos := playground.GetRepositories()
	var repoDTOs []models.CandleRepositoryDTO
	for _, repo := range repos {
		repoDTOs = append(repoDTOs, repo.ToDTO())
	}

	store := &models.PlaygroundSession{
		ID:              playground.GetId(),
		ClientID:        playground.GetClientId(),
		CurrentTime:     playground.GetCurrentTime(),
		StartAt:         meta.StartAt,
		EndAt:           meta.EndAt,
		Balance:         playground.GetBalance(),
		StartingBalance: meta.InitialBalance,
		Repositories:    models.CandleRepositoryRecord(repoDTOs),
		Tags:            meta.Tags,
		Env:             string(meta.Environment),
	}

	// todo: only needed for reconcile playgrounds.
	// live playgrounds can be used to reference reconcile playground source
	if meta.Environment == models.PlaygroundEnvironmentLive {
		if meta.SourceBroker == "" || meta.SourceAccountId == "" {
			return nil, errors.New("savePlaygroundSession: missing broker, or account id")
		}

		if err := meta.LiveAccountType.Validate(); err != nil {
			return nil, fmt.Errorf("savePlaygroundSession: invalid live account type: %w", err)
		}

		reconcilePlayground := playground.GetReconcilePlayground()
		reconcilePlaygroundID := reconcilePlayground.GetId()
		liveAccount := models.LiveAccount{
			BrokerName:            meta.SourceBroker,
			AccountId:             meta.SourceAccountId,
			AccountType:           meta.LiveAccountType,
			ReconcilePlayground:   reconcilePlayground,
			ReconcilePlaygroundID: reconcilePlaygroundID,
		}

		if err := tx.FirstOrCreate(&liveAccount, models.LiveAccount{
			BrokerName:            meta.SourceBroker,
			AccountId:             meta.SourceAccountId,
			AccountType:           meta.LiveAccountType,
			ReconcilePlaygroundID: reconcilePlaygroundID,
		}).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch or create live account: %w", err)
		}

		store.LiveAccount = &liveAccount
		store.Broker = &meta.SourceBroker
		store.AccountID = &meta.SourceAccountId

		var liveAccountType *string
		if err := meta.LiveAccountType.Validate(); err == nil {
			val := string(meta.LiveAccountType)
			liveAccountType = &val
		}
		store.LiveAccountType = liveAccountType
	}

	if err := tx.Create(store).Error; err != nil {
		return nil, fmt.Errorf("failed to save playground: %w", err)
	}

	return store, nil
}

func (s *DatabaseService) SavePlaygroundSession(playground models.IPlayground) (*models.PlaygroundSession, error) {
	return savePlaygroundSessionTx(db, playground)
}

func (s *DatabaseService) SaveOrderRecord(playgroundId uuid.UUID, order *models.BacktesterOrder, newBalance *float64, liveAccountType models.LiveAccountType) (*models.OrderRecord, error) {
	var oRec *models.OrderRecord
	
	err := db.Transaction(func(tx *gorm.DB) error {
		var oRecs []*models.OrderRecord
		var e error
		if oRecs, e = saveOrderRecordsTx(tx, playgroundId, []*models.BacktesterOrder{order}, &liveAccountType); e != nil {
			return fmt.Errorf("saveOrderRecord: failed to save order records: %w", e)
		}

		if len(oRecs) != 1 {
			return fmt.Errorf("saveOrderRecord: expected 1 order record, got %d", len(oRecs))
		}

		oRec = oRecs[0]

		if newBalance != nil {
			if err := saveBalance(tx, playgroundId, *newBalance); err != nil {
				return fmt.Errorf("saveOrderRecord: failed to save balance: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("saveOrderRecord: save order record transaction failed: %w", err)
	}

	return oRec, nil
}
