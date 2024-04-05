package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	"github.com/google/uuid"
)

func main() {
	eventStoreDbURL := "esdb+discover://localhost:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000"
	// eventStoreDbURL := "esdb+discover://dchome.loclx.io?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000"
	streamName1 := "option-alerts"
	streamName2 := "accounts"
	streamName3 := "option-chain-ticks"
	streamName4 := "stock-ticks"
	settings, err := esdb.ParseConnectionString(eventStoreDbURL)

	if err != nil {
		panic(err)
	}

	db, err := esdb.NewClient(settings)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Which stream would you like to read from?\n1: %s\n2: %s\n3: %s\n4: %s\n", streamName1, streamName2, streamName3, streamName4)
	var streamName string
	if _, err := fmt.Scanln(&streamName); err != nil {
		panic(err)
	}

	if streamName == "1" {
		streamName = streamName1
	} else if streamName == "2" {
		streamName = streamName2
	} else if streamName == "3" {
		streamName = streamName3
	} else if streamName == "4" {
		streamName = streamName4
	} else {
		fmt.Println("Invalid stream name")
		return
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
	// stream, err := db.ReadStream(context.Background(), streamName, esdb.ReadStreamOptions{}, 12)
	// if err != nil {
	// 	panic(err)
	// }
	subscription, err := db.SubscribeToStream(context.Background(), streamName, esdb.SubscribeToStreamOptions{
		From: esdb.Start{},
	})

	if err != nil {
		panic(err)
	}

	// defer stream.Close()

	for {
		// event, err := stream.Recv()
		event := subscription.Recv()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			panic(err)
		}

		ev := event.EventAppeared.Event

		// Doing something productive with the event
		fmt.Println(ev.EventNumber)
		fmt.Println(ev.EventType)
		fmt.Println(ev.EventID)

		var data map[string]interface{}
		err = json.Unmarshal(ev.Data, &data)
		if err != nil {
			panic(err)
		}

		fmt.Println(data)

		if reqId, found := data["requestID"]; found {
			id, err := uuid.Parse(reqId.(string))
			if err == nil {
				// Convert to decimal
				fmt.Printf("requestID (decimal): %d\n", id)
			} else {
				fmt.Println("error:", err)
				continue
			}
		}

		fmt.Println("--------------------")
	}
}
