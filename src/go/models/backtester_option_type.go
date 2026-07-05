package models

type BacktesterOptionType string

const (
	BacktesterOptionTypeCall BacktesterOptionType = "C"
	BacktesterOptionTypePut  BacktesterOptionType = "P"
)
