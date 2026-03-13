package eventmodels

type RepositorySourceType string

const (
	RepositorySourcePolygon RepositorySourceType = "polygon"
	RepositorySourceCSV     RepositorySourceType = "csv"
	RepositorySourceTradier RepositorySourceType = "tradier"
)

type RepositorySource struct {
	Type        RepositorySourceType `json:"type"`
	CSVFilename *string              `json:"filename"`
}

type CreateRepositoryRequest struct {
	Symbol        string                 `json:"symbol"`
	Timespan      PolygonTimespanRequest `json:"timespan"`
	HistoryInDays uint32                 `json:"history_in_days"`
	Source        RepositorySource       `json:"source"`
	Indicators    []string               `json:"indicators"`
}
