package eventmodels

import "time"

type CsvCandle struct {
	SavedEventParms SavedEventParameters `json:"-"`
	Timestamp       time.Time
	Open            float64
	High            float64
	Low             float64
	Close           float64
	UpTrendBegins   float64
	DownTrendBegins float64
}

func (dto *CsvCandle) GetSavedEventParameters() SavedEventParameters {
	return dto.SavedEventParms
}

func (dto *CsvCandle) GetMetaData() *MetaData {
	return &MetaData{}
}

func NewCsvCandle(streamName StreamName, eventName EventName, schemaVersion int) *CsvCandle {
	return &CsvCandle{
		SavedEventParms: SavedEventParameters{
			StreamName:    streamName,
			EventName:     eventName,
			SchemaVersion: schemaVersion,
		},
	}
}
