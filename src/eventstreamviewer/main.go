package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventservices"
)

func processEvent(ev *esdb.RecordedEvent) {
	// Doing something productive with the event
	fmt.Println(ev.EventNumber)
	fmt.Println(ev.EventType)
	fmt.Println(ev.EventID)

	var data map[string]interface{}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
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
			return
		}
	}

	fmt.Println("--------------------")
}

func main() {
	ctx := context.Background()
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")

	settings, err := esdb.ParseConnectionString(eventStoreDbURL)

	if err != nil {
		panic(err)
	}

	esdbClient, err := esdb.NewClient(settings)
	if err != nil {
		panic(err)
	}

	streamNames := eventservices.ListAllStreams(ctx, esdbClient)

	var msg strings.Builder
	msg.WriteString("Which stream would you like to read from?\n")

	for i, streamName := range streamNames {
		msg.WriteString(fmt.Sprintf("%d: %s\n", i+1, streamName))
	}

	fmt.Printf("%s", msg.String())
	var streamName string
	if _, err := fmt.Scanln(&streamName); err != nil {
		panic(fmt.Errorf("failed to read stream name: %w", err))
	}

	streamNameIndex, err := strconv.Atoi(streamName)
	if err != nil {
		panic(fmt.Errorf("invalid stream index: %w", err))
	}

	if streamNameIndex < 1 || streamNameIndex > len(streamNames) {
		panic(fmt.Errorf("invalid stream index: %w", err))
	}

	streamName = string(streamNames[streamNameIndex-1])

	fmt.Printf("Would you like to delete the stream %s? (y/n)\n", streamName)
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		panic(err)
	}

	if strings.ToLower(input) == "y" {
		_, err := esdbClient.DeleteStream(context.Background(), streamName, esdb.DeleteStreamOptions{})
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
	subscription, err := esdbClient.SubscribeToStream(ctx, streamName, esdb.SubscribeToStreamOptions{
		From: esdb.Start{},
	})

	if err != nil {
		log.Panicf("eventStoreDBClient: failed to subscribe to stream: %v", err)
	}

	log.Infof("subscribed to stream %s", streamName)

	var lastEventNumber uint64 = 0

	for {
		for {
			fmt.Println("A")

			event := subscription.Recv()
			fmt.Println("B")

			if event.SubscriptionDropped != nil {
				log.Infof("Subscription dropped: %v", event.SubscriptionDropped.Error)
				break
			}

			if event.EventAppeared == nil {
				continue
			}

			if event.CheckPointReached != nil {
				fmt.Printf("checkpoint reached: %v\n", event.CheckPointReached)
			}

			ev := event.EventAppeared.Event

			// lastEventNumber = event.EventAppeared.OriginalEvent().EventNumber
			fmt.Printf("lastEventNumber: %d\n", lastEventNumber)
			processEvent(ev)
		}

		log.Infof("re-subscribing subscription @ pos %v", lastEventNumber)

		subscription, err = esdbClient.SubscribeToStream(ctx, streamName, esdb.SubscribeToStreamOptions{
			From: esdb.Revision(lastEventNumber),
		})

		if err != nil {
			log.Panicf("eventStoreDBClient: failed to subscribe to stream: %v", err)
		}
	}
}
