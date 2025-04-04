package models

import "fmt"

type PlaygroundEnvironment string

func (e PlaygroundEnvironment) Validate() error {
	switch e {
	case PlaygroundEnvironmentSimulator, PlaygroundEnvironmentLive, PlaygroundEnvironmentReconcile:
		return nil
	default:
		return fmt.Errorf("invalid playground environment: %s", e)
	}
}

const (
	PlaygroundEnvironmentSimulator PlaygroundEnvironment = "simulator"
	PlaygroundEnvironmentLive      PlaygroundEnvironment = "live"
	PlaygroundEnvironmentReconcile PlaygroundEnvironment = "reconcile"
)
