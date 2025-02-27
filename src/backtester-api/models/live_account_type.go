package models

import "fmt"

type LiveAccountType string

const (
	LiveAccountTypePaper         LiveAccountType = "paper"
	LiveAccountTypeMargin        LiveAccountType = "margin"
	LiveAccountTypeReconcilation LiveAccountType = "reconcilation"
	LiveAccountTypeMock          LiveAccountType = "mock"
)

func (t LiveAccountType) Validate() error {
	switch t {
	case LiveAccountTypePaper:
		break
	case LiveAccountTypeMargin:
		break
	default:
		return fmt.Errorf("LiveAccountType: unsupported account type: %s", t)
	}

	return nil
}
