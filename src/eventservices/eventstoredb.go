package eventservices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func CalculateStreamSize(ctx context.Context, esdbClient *esdb.Client, streamName eventmodels.StreamName) (int64, error) {
	var size int64
	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}

	count := 0
	fetchSize := 4096
	lastEventNo, err := FindStreamLastEventNumber(ctx, esdbClient, streamName)
	if err != nil {
		return 0, fmt.Errorf("failed to find last event number: %v", err)
	}

	if lastEventNo == 0 {
		return 0, nil
	}

	terminalEventNumber := int(lastEventNo)

	for count < terminalEventNumber {
		stream, err := esdbClient.ReadStream(ctx, string(streamName), readOptions, uint64(fetchSize))
		if err != nil {
			return 0, err
		}
		defer stream.Close()

		for {
			event, err := stream.Recv()
			if err != nil {
				break
			}
			size += int64(len(event.Event.Data))
			size += int64(len(event.Event.UserMetadata))
			size += int64(len(event.Event.SystemMetadata))
		}

		count += fetchSize
	}

	return size, nil
}

func FetchAllData[T eventmodels.SavedEvent](ctx context.Context, esdbClient *esdb.Client, instance T) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0)
	var currentEventNumber uint64

	params := instance.GetSavedEventParameters()

	lastEventNumber, err := FindStreamLastEventNumber(ctx, esdbClient, params.StreamName)
	if err != nil {
		return nil, fmt.Errorf("FetchAllData: failed to find last event number: %w", err)
	}

	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}

	for {
		stream, err := esdbClient.ReadStream(ctx, string(params.StreamName), readOptions, 4096)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("FetchAllData: failed to read stream %s: %w", params.StreamName, err)
		}
		defer stream.Close()

		for {
			event, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				if esdbError, ok := err.(*esdb.Error); ok && esdbError.IsErrorCode(esdb.ErrorCodeResourceNotFound) {
					break
				}

				return nil, fmt.Errorf("FetchAllData: failed to read event from stream: %w", err)
			}

			var object map[string]interface{}
			if err := json.Unmarshal(event.Event.Data, &object); err != nil {
				return nil, fmt.Errorf("FetchAllData: failed to unmarshal event data: %w", err)
			}

			currentEventNumber = event.Event.EventNumber

			results = append(results, object)
		}

		if currentEventNumber == lastEventNumber {
			break
		}

		readOptions.From = esdb.Revision(currentEventNumber)
	}

	return results, nil
}

func FetchAll[T eventmodels.SavedEvent](ctx context.Context, esdbClient *esdb.Client, instance T) ([]T, error) {
	results := []T{}
	var currentEventNumber uint64

	params := instance.GetSavedEventParameters()

	lastEventNumber, err := FindStreamLastEventNumber(ctx, esdbClient, params.StreamName)
	if err != nil {
		return nil, fmt.Errorf("FetchAll: failed to find last event number: %w", err)
	}

	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}

	for {
		stream, err := esdbClient.ReadStream(ctx, string(params.StreamName), readOptions, 4096)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("FetchAll: failed to read stream %s: %w", params.StreamName, err)
		}
		defer stream.Close()

		for {
			event, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				if esdbError, ok := err.(*esdb.Error); ok && esdbError.IsErrorCode(esdb.ErrorCodeResourceNotFound) {
					break
				}

				return nil, fmt.Errorf("FetchAll: failed to read event from stream: %w", err)
			}

			var object T
			if err := json.Unmarshal(event.Event.Data, &object); err != nil {
				return nil, fmt.Errorf("FetchAll: failed to unmarshal event data: %w, data=%s", err, event.Event.Data)
			}

			currentEventNumber = event.Event.EventNumber

			results = append(results, object)
		}

		if currentEventNumber == lastEventNumber {
			break
		}

		readOptions.From = esdb.Revision(currentEventNumber)
	}

	return results, nil
}

func FetchAllDeprecated[T eventmodels.SavedEvent](ctx context.Context, esdbClient *esdb.Client, instance T) (map[eventmodels.EventStreamID]T, error) {
	panic("not implemented")
	// results := make(map[eventmodels.EventStreamID]T)
	// var currentEventNumber uint64

	// params := instance.GetSavedEventParameters()

	// lastEventNumber, err := FindStreamLastEventNumber(ctx, esdbClient, params.StreamName)
	// if err != nil {
	// 	return nil, fmt.Errorf("FetchAll: failed to find last event number: %w", err)
	// }

	// readOptions := esdb.ReadStreamOptions{
	// 	Direction: esdb.Forwards,
	// 	From:      esdb.Start{},
	// }

	// for {
	// 	stream, err := esdbClient.ReadStream(ctx, string(params.StreamName), readOptions, 4096)
	// 	if err != nil {
	// 		if errors.Is(err, io.EOF) {
	// 			break
	// 		}

	// 		return nil, fmt.Errorf("FetchAll: failed to read stream %s: %w", params.StreamName, err)
	// 	}
	// 	defer stream.Close()

	// 	for {
	// 		event, err := stream.Recv()
	// 		if err != nil {
	// 			if errors.Is(err, io.EOF) {
	// 				break
	// 			}

	// 			if esdbError, ok := err.(*esdb.Error); ok && esdbError.IsErrorCode(esdb.ErrorCodeResourceNotFound) {
	// 				break
	// 			}

	// 			return nil, fmt.Errorf("FetchAll: failed to read event from stream: %w", err)
	// 		}

	// 		var object T
	// 		if err := json.Unmarshal(event.Event.Data, &object); err != nil {
	// 			return nil, fmt.Errorf("FetchAll: failed to unmarshal event data: %w", err)
	// 		}

	// 		currentEventNumber = event.Event.EventNumber

	// 		results[object.GetMetaData().GetEventStreamID()] = object
	// 	}

	// 	if currentEventNumber == lastEventNumber {
	// 		break
	// 	}

	// 	readOptions.From = esdb.Revision(currentEventNumber)
	// }

	// return results, nil
}

func FindStreamLastEventNumber(ctx context.Context, db *esdb.Client, streamName eventmodels.StreamName) (uint64, error) {
	stream, err := db.ReadStream(ctx, string(streamName), esdb.ReadStreamOptions{
		Direction: esdb.Backwards,
		From:      esdb.End{},
	}, 1)

	if err != nil {
		if esdbError, ok := err.(*esdb.Error); ok && esdbError.IsErrorCode(esdb.ErrorCodeResourceNotFound) {
			return 0, nil
		}

		return 0, fmt.Errorf("FindStreamLastEventNumber: failed to read stream %s: %w", streamName, err)
	}

	event, err := stream.Recv()
	if err != nil {
		if esdbError, ok := err.(*esdb.Error); ok && esdbError.IsErrorCode(esdb.ErrorCodeResourceNotFound) {
			return 0, nil
		}

		return 0, fmt.Errorf("FindStreamLastEventNumber: failed to read event from stream %s: %w", streamName, err)
	}

	return event.Event.EventNumber, nil
}

func ListAllStreams(ctx context.Context, esdbClient *esdb.Client) []eventmodels.StreamName {
	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}
	stream, err := esdbClient.ReadStream(ctx, "$streams", readOptions, 4096)

	if err != nil {
		log.Fatalf("Failed to read from $streams: %v", err)
	}
	defer stream.Close()

	streams := make([]eventmodels.StreamName, 0)
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}
		name := string(event.Event.Data)[2:]
		if strings.HasPrefix(name, "$$") {
			continue
		}

		streams = append(streams, eventmodels.StreamName(name))
	}

	return streams
}
