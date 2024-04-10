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

func FetchAllOptionContracts(ctx context.Context, esdbClient *esdb.Client) (map[string]eventmodels.OptionContract, error) {
	results := make(map[string]eventmodels.OptionContract)
	var currentEventNumber uint64

	lastEventNumber, err := FindStreamLastEventNumber(esdbClient, string(eventmodels.OptionContractStream))
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

func FindStreamLastEventNumber(db *esdb.Client, streamName string) (uint64, error) {
	stream, err := db.ReadStream(context.Background(), streamName, esdb.ReadStreamOptions{
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

func ListAllStreams(ctx context.Context, esdbClient *esdb.Client) []string {
	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}
	stream, err := esdbClient.ReadStream(ctx, "$streams", readOptions, 4096)
	if err != nil {
		log.Fatalf("Failed to read from $streams: %v", err)
	}
	defer stream.Close()

	streams := make([]string, 0)
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}
		streamName := string(event.Event.Data)[2:]
		if strings.HasPrefix(streamName, "$$") {
			continue
		}

		streams = append(streams, streamName)
	}

	return streams
}
