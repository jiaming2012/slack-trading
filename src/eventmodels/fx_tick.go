package eventmodels

import "time"

type FxTick struct {
	Symbol    FxSymbol  `json:"-"`
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
}

func (t *FxTick) GetMetaData() *MetaData {
	return &MetaData{}
}

func (t *FxTick) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    NewFxTickStreamName(t.Symbol),
		EventName:     FxTickSavedEvent,
		SchemaVersion: 1,
	}
}
