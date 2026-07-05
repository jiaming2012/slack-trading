package telemetry

import (
	log "github.com/sirupsen/logrus"
)

// errorCounterHook increments the registry's error counter for every log
// entry at Error level or above. This replaces log-pipeline-based alerting
// (ADR-0005); log storage itself is an explicit v1 non-goal.
type errorCounterHook struct{}

// NewErrorCounterHook returns the logrus hook to register in main.
func NewErrorCounterHook() log.Hook {
	return errorCounterHook{}
}

func (errorCounterHook) Levels() []log.Level {
	return []log.Level{log.ErrorLevel, log.FatalLevel, log.PanicLevel}
}

func (errorCounterHook) Fire(*log.Entry) error {
	ErrorsTotal.Add(1)
	return nil
}
