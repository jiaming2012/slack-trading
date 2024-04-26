package eventmodels

type SavedEvent interface {
	GetSavedEventParameters() SavedEventParameters
	GetMetaData() MetaData
	SetEventStreamID(id uint64)
	GetEventStreamID() uint64
}
