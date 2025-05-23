package data

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func saveOrderRecordsTx(_db *gorm.DB, orders []*models.OrderRecord, forceNew bool) error {
	// var allOrderRecords []*models.OrderRecord
	// var updateOrderRequests []*models.UpdateOrderRecordRequest

	err := _db.Transaction(func(tx *gorm.DB) error {
		for _, order := range orders {
			var e error

			if e = order.Validate(); e != nil {
				return fmt.Errorf("failed to validate order: %w", e)
			}

			if forceNew {
				if e = tx.Create(order).Error; e != nil {
					return fmt.Errorf("failed to create order records: %w", e)
				}
			} else {
				if order.ID == 0 {
					return fmt.Errorf("saveOrderRecordsTx: order ID is 0")
				}

				var existing models.OrderRecord
				if err := tx.First(&existing, order.ID).Error; err != nil {
					return fmt.Errorf("saveOrderRecordsTx: failed to find existing order: %w", err)
				}

				if err := tx.Save(order).Error; err != nil {
					return fmt.Errorf("saveOrderRecordsTx: failed to update order: %w", err)
				}
			}
		}

		return nil
	})

	return err
}

func saveBalance(tx *gorm.DB, playgroundId uuid.UUID, balance float64) error {
	if result := tx.Model(&models.Playground{}).Where("id = ?", playgroundId).Update("balance", balance); result.Error != nil {
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

func savePlaygroundTx(tx *gorm.DB, playground *models.Playground) error {
	meta := playground.GetMeta()

	if err := meta.Validate(); err != nil {
		return fmt.Errorf("savePlaygroundSession: invalid playground meta: %w", err)
	}

	if err := tx.Create(playground).Error; err != nil {
		return fmt.Errorf("failed to save playground: %w", err)
	}

	return nil
}
