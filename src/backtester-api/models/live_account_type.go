package models

import "fmt"

type LiveAccountType string

const (
	LiveAccountTypePaper         LiveAccountType = "paper"
	LiveAccountTypeMargin        LiveAccountType = "margin"
	LiveAccountTypeReconcilation LiveAccountType = "reconcilation"
	LiveAccountTypeMock          LiveAccountType = "mock"
	LiveAccountTypeSimulator     LiveAccountType = "simulator"
)

func (t LiveAccountType) Validate() error {
	switch t {
	case LiveAccountTypePaper:
		break
	case LiveAccountTypeMargin:
		break
	case LiveAccountTypeReconcilation:
		break
	case LiveAccountTypeMock:
		break
	case LiveAccountTypeSimulator:
		break
	default:
		return fmt.Errorf("LiveAccountType: unsupported account type: %s", t)
	}

	return nil
}
