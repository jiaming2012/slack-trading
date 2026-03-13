package eventmodels

type SavedEvent interface {
	GetSavedEventParameters() SavedEventParameters
	GetMetaData() *MetaData // todo: remove from SavedEvent. This is more of for request events
}
