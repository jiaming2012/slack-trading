package models

import "fmt"

// OrderSaveIntent is a transaction-free order persistence request: the order
// record to save plus its force-new flag. Intents are staged by the broker
// adapters and persisted by the database layer inside one store-owned
// transaction — no *gorm.DB crosses the package boundary.
type OrderSaveIntent struct {
	Order    *OrderRecord
	ForceNew bool
}

// PlaceOrderChanges describes one step of committing a placed order: an
// optional in-memory mutation (Commit) plus zero or more staged order-save
// intents that the database layer persists atomically.
type PlaceOrderChanges struct {
	Commit      func() error
	SaveIntents []OrderSaveIntent
	Info        string
}

// CommitPlaceOrderChanges applies the in-memory commits in order, then hands
// every staged order-save intent to the database layer in a single batch. The
// store owns the transaction, so all intents are persisted or none are.
func CommitPlaceOrderChanges(db IDatabaseService, changes []*PlaceOrderChanges) error {
	var intents []OrderSaveIntent

	for _, change := range changes {
		if change == nil {
			continue
		}

		if change.Commit != nil {
			if err := change.Commit(); err != nil {
				return fmt.Errorf("failed to commit change (%s): %w", change.Info, err)
			}
		}

		intents = append(intents, change.SaveIntents...)
	}

	if len(intents) == 0 {
		return nil
	}

	if err := db.SaveOrderRecordIntents(intents); err != nil {
		return fmt.Errorf("failed to save order record intents: %w", err)
	}

	return nil
}
