package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventservices"
	"slack-trading/src/utils"
)

func printInAlphebeticalOrder(data map[string]interface{}) {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("%s: %v\n", k, data[k])
	}
}

func processEvent(ev *esdb.RecordedEvent) {
	// Doing something productive with the event
	fmt.Printf("EventStreamID: %v\n", ev.EventNumber)
	fmt.Println(ev.EventType)
	// fmt.Println(ev.EventID)

	var data map[string]interface{}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		panic(err)
	}

	printInAlphebeticalOrder(data)

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
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	goEnv := os.Getenv("GO_ENV")
	if goEnv == "" {
		panic("missing GO_ENV environment variable")
	}

	ctx, cancel := context.WithCancel(context.Background())

	utils.InitEnvironmentVariables(projectsDir, goEnv)

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

	fmt.Println("Enter number of events to read (Hit ENTER to read all): ")
	var numEvents int

	fmt.Scanln(&numEvents)

	// ---- read event ----
	subscription, err := esdbClient.SubscribeToStream(ctx, streamName, esdb.SubscribeToStreamOptions{
		From: esdb.Start{},
	})

	if err != nil {
		log.Panicf("eventStoreDBClient: failed to subscribe to stream: %v", err)
	}

	log.Infof("subscribed to stream %s", streamName)

	var lastEventNumber uint64 = 0
	eventsRead := 0

	for {
		for {
			event := subscription.Recv()

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

			lastEventNumber = event.EventAppeared.OriginalEvent().EventNumber

			processEvent(ev)

			if numEvents > 0 && eventsRead >= numEvents-1 {
				cancel()
				os.Exit(0)
			}

			eventsRead += 1
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
