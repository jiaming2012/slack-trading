package eventservices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/EventStore/EventStore-Client-Go/esdb"

	"slack-trading/src/eventmodels"
)

func CalculateStreamSize(ctx context.Context, esdbClient *esdb.Client, streamName eventmodels.StreamName) (int64, error) {
	var size int64
	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}

	count := 0
	fetchSize := 4096
	lastEventNo, err := FindStreamLastEventNumber(esdbClient, streamName)
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

func FetchAllOptionContracts(ctx context.Context, esdbClient *esdb.Client) (map[string]eventmodels.OptionContract, error) {
	results := make(map[string]eventmodels.OptionContract)
	var currentEventNumber uint64

	lastEventNumber, err := FindStreamLastEventNumber(esdbClient, eventmodels.OptionContractStream)
	if err != nil {
		return nil, fmt.Errorf("failed to find last event number: %w", err)
	}

	if lastEventNumber == 0 {
		return results, nil
	}

	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}

	for {
		stream, err := esdbClient.ReadStream(ctx, string(eventmodels.OptionContractStream), readOptions, 4096)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("failed to read stream %s: %w", eventmodels.OptionContractStream, err)
		}
		defer stream.Close()

		for {
			event, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, fmt.Errorf("failed to read event from stream: %w", err)
			}

			var contract eventmodels.OptionContract
			if err := json.Unmarshal(event.Event.Data, &contract); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
			}

			currentEventNumber = event.Event.EventNumber

			contract.ID = eventmodels.OptionContractID(currentEventNumber)

			results[contract.Symbol] = contract
		}

		if currentEventNumber == lastEventNumber {
			break
		}

		readOptions.From = esdb.Revision(currentEventNumber)
	}

	return results, nil
}

func FindStreamLastEventNumber(db *esdb.Client, streamName eventmodels.StreamName) (uint64, error) {
	stream, err := db.ReadStream(context.Background(), string(streamName), esdb.ReadStreamOptions{
		Direction: esdb.Backwards,
		From:      esdb.End{},
	}, 1)

	if err != nil {
		if errors.Is(err, esdb.ErrStreamNotFound) {
			return 0, nil
		}

		return 0, fmt.Errorf("failed to read stream %s: %w", streamName, err)
	}

	event, err := stream.Recv()
	if err != nil {
		return 0, fmt.Errorf("failed to read event from stream %s: %w", streamName, err)
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
