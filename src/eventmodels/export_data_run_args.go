package eventmodels

import "time"

type ExportDataRunArgs struct {
	InputStreamName string
	StartsAt        time.Time
	EndsAt          time.Time
	GoEnv           string
}
