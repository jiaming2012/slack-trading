package eventstore

import (
	"context"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"

	"slack-trading/src/eventmodels"
)

func InsertEvent(ctx context.Context, eventName eventmodels.EventName, streamName string, eventType string, data []byte, db *esdb.Client) error {
	eventData := esdb.EventData{
		ContentType: esdb.ContentTypeJson,
		EventType:   string(eventName),
		Data:        data,
	}

	_, err := db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)

	return err
}
