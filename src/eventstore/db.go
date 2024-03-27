package eventstore

import (
	"context"

	"github.com/EventStore/EventStore-Client-Go/esdb"

	"slack-trading/src/eventmodels"
)

func InsertEvent(ctx context.Context, eventName eventmodels.EventName, streamName string, eventType string, data []byte, db *esdb.Client) error {
	eventData := esdb.EventData{
		ContentType: esdb.JsonContentType,
		EventType:   string(eventName),
		Data:        data,
	}

	_, err := db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)

	return err
}
