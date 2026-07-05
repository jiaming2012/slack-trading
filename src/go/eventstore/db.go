package eventstore

import (
	"context"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

func InsertEvent(ctx context.Context, eventName models.EventName, streamName string, eventType string, data []byte, db *esdb.Client) error {
	eventData := esdb.EventData{
		ContentType: esdb.ContentTypeJson,
		EventType:   string(eventName),
		Data:        data,
	}

	_, err := db.AppendToStream(ctx, streamName, esdb.AppendToStreamOptions{}, eventData)

	return err
}
