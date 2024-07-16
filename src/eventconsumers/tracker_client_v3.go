package eventconsumers

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type TrackerV3Client = esdbConsumerStream[*eventmodels.TrackerV3]
