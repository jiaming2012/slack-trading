package eventmodels

import "fmt"

type SignalConstraints []*ExitSignalConstraint

func (constraints SignalConstraints) Validate() error {
	names := make(map[string]struct{})
	for _, c := range constraints {
		if _, exists := names[c.Name]; exists {
			return fmt.Errorf("SignalConstraints.Validate: duplicate name not allowed, found %v twice", c.Name)
		}
		names[c.Name] = struct{}{}
	}

	return nil
}
