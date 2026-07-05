package eventconsumers

import "github.com/jiaming2012/slack-trading/src/go/models"

type TrackerV3Client = esdbConsumerStream[*models.TrackerV3]
