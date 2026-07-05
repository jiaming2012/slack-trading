package models

import "fmt"

// These sentinel errors were defined in the pre-merge legacy models package and
// are consumed by the live backtester service. The canonical (eventmodels-sourced)
// error.go did not carry them, so they are retained here
// (reconcile-models-packages).
var ErrOptionContractIsExpired = fmt.Errorf("option contract is expired")
var ErrNoCandlesFound = fmt.Errorf("no candles found")
