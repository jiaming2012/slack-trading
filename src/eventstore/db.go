package eventstore

import (
	"context"

	"github.com/EventStore/EventStore-Client-Go/esdb"

	pubsub "slack-trading/src/eventpubsub"
)

func InsertEvent(ctx context.Context, eventName pubsub.EventName, streamName string, eventType string, data []byte, db *esdb.Client) error {
	eventData := esdb.EventData{
		ContentType: esdb.JsonContentType,
		EventType:   string(eventName),
		Data:        data,
	}

	_, err := db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)

	return err
}
