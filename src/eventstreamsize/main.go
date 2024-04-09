package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	log "github.com/sirupsen/logrus"
)

func listAllStreams(ctx context.Context, esdbClient *esdb.Client) []string {
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

func getStreamSize(ctx context.Context, esdbClient *esdb.Client) {
	streamNames := listAllStreams(ctx, esdbClient)

	for _, streamName := range streamNames {
		size, err := calculateStreamSize(ctx, esdbClient, streamName)
		if err != nil {
			log.Errorf("Error calculating size for stream %s: %v", streamName, err)
			continue
		}

		sizeInMb := float64(size) / 1024 / 1024

		fmt.Printf("Stream: %s, Size: %.2f MB\n", streamName, sizeInMb)
	}
}

func main() {
	// Set the connection details
	config, err := esdb.ParseConnectionString("esdb://localhost:2113?tls=false")
	if err != nil {
		log.Fatalf("Error parsing connection string: %v", err)
	}

	// Create a new client
	esdbClient, err := esdb.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer esdbClient.Close()

	fmt.Printf("Enter a command:\n1. List all streams\n2. Calculate all stream sizes\n")
	var command int
	fmt.Scanln(&command)
	fmt.Printf("***********************\n")

	ctx := context.Background()

	switch command {
	case 1:
		streams := listAllStreams(ctx, esdbClient)
		for _, stream := range streams {
			fmt.Println(stream)
		}
	case 2:
		getStreamSize(ctx, esdbClient)
	default:
		log.Fatalf("Invalid command: %d", command)
	}
}

func findStreamLastEventNumber(db *esdb.Client, streamName string) uint64 {
	stream, err := db.ReadStream(context.Background(), streamName, esdb.ReadStreamOptions{
		Direction: esdb.Backwards,
		From:      esdb.End{},
	}, 1)

	if err != nil {
		panic(err)
	}

	event, err := stream.Recv()
	if err != nil {
		panic(err)
	}

	return event.Event.EventNumber
}

func calculateStreamSize(ctx context.Context, esdbClient *esdb.Client, streamName string) (int64, error) {
	var size int64
	readOptions := esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Start{},
	}

	count := 0
	fetchSize := 4096
	terminalEventNumber := int(findStreamLastEventNumber(esdbClient, streamName))

	for count < terminalEventNumber {
		stream, err := esdbClient.ReadStream(ctx, streamName, readOptions, uint64(fetchSize))
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
