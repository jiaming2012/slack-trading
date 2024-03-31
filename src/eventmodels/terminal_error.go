package eventmodels

type TerminalError struct {
	Error error
	Meta  MetaData
}

func (e *TerminalError) GetMetaData() MetaData {
	return e.Meta
}

func (e *TerminalError) SetMetaData(meta *MetaData) {
	e.Meta = *meta
}

func NewTerminalError(meta *MetaData, err error) *TerminalError {
	return &TerminalError{
		Error: err,
		Meta:  *meta,
	}
}
