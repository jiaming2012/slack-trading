package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/cmd/telemetry"
)

func main() {
	if err := telemetry.RunQuickstart(); err != nil {
		log.Fatalln(err)
	}
}
