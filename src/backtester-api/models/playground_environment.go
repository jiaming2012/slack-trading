package models

import "fmt"

type PlaygroundEnvironment string

func (e PlaygroundEnvironment) Validate() error {
	switch e {
	case PlaygroundEnvironmentSimulator, PlaygroundEnvironmentLive:
		return nil
	default:
		return fmt.Errorf("invalid playground environment")
	}
}

const (
	PlaygroundEnvironmentSimulator PlaygroundEnvironment = "simulator"
	PlaygroundEnvironmentLive      PlaygroundEnvironment = "live"
)
