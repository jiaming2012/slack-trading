package models

import "fmt"

var (
	ErrInsufficientFreeMargin        = fmt.Errorf("insufficient free margin")
	ErrInvalidOrderVolumeLongVolume  = fmt.Errorf("invalid order volume: cannot close more than long volume")
	ErrInvalidOrderVolumeShortVolume = fmt.Errorf("invalid order volume: cannot close more than short volume")
	ErrInvalidOrderVolumeZero        = fmt.Errorf("invalid order volume: cannot close zero volume")
	ErrNoPriceAvailable              = fmt.Errorf("no price available")
)
