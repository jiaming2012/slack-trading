package eventmodels

type SavedEvent interface {
	GetSavedEventParameters() SavedEventParameters
	GetMetaData() *MetaData
}
