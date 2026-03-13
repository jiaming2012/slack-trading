package eventmodels

type ExitSignalConstraint struct {
	Name  string                                                                  `json:"name"`
	Check func(*PriceLevel, *ExitCondition, map[string]interface{}) (bool, error) `json:"-"`
}

func NewExitSignalConstraint(name string, check func(level *PriceLevel, condition *ExitCondition, params map[string]interface{}) (bool, error)) *ExitSignalConstraint {
	return &ExitSignalConstraint{Name: name, Check: check}
}
