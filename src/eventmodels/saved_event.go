package eventmodels

type SavedEvent interface {
	GetSavedEventParameters() SavedEventParameters
}
