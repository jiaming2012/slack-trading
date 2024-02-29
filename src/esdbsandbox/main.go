package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/EventStore/EventStore-Client-Go/esdb"
)

func main() {
	eventStoreDbURL := "esdb+discover://localhost:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000"
	streamName := "accounts"
	settings, err := esdb.ParseConnectionString(eventStoreDbURL)

	if err != nil {
		panic(err)
	}

	db, err := esdb.NewClient(settings)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Would you like to delete the stream %s? (y/n)\n", streamName)
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		panic(err)
	}

	if strings.ToLower(input) == "y" {
		_, err := db.DeleteStream(context.Background(), streamName, esdb.DeleteStreamOptions{})
		if err != nil {
			panic(err)
		}

		fmt.Printf("%s Deleted\n", streamName)
	}

	// ---- read event ----
	stream, err := db.ReadStream(context.Background(), streamName, esdb.ReadStreamOptions{}, 10)
	if err != nil {
		panic(err)
	}

	defer stream.Close()

	for {
		event, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			panic(err)
		}

		// Doing something productive with the event
		fmt.Println(event.Event.EventNumber)
		fmt.Println(event.Event.EventType)
		fmt.Println(event.Event.EventID)

		var data map[string]interface{}
		err = json.Unmarshal(event.Event.Data, &data)
		if err != nil {
			panic(err)
		}

		fmt.Println(data)
	}
}
