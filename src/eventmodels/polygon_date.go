package eventmodels

import (
	"fmt"
	"time"
)

type PolygonDate struct {
	Year  int
	Month int
	Day   int
}

func (d *PolygonDate) ToString() string {
	return fmt.Sprintf("%d-%02d-%02d", d.Year, d.Month, d.Day)
}

func (d *PolygonDate) ToTime() (time.Time, error) {
	return time.Parse("2006-01-02", d.ToString())
}

func (d *PolygonDate) GetPreviousDay() *PolygonDate {
	date := time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
	previousDay := date.AddDate(0, 0, -1)
	return &PolygonDate{
		Year:  previousDay.Year(),
		Month: int(previousDay.Month()),
		Day:   previousDay.Day(),
	}
}

func (d *PolygonDate) GetPreviousYear() *PolygonDate {
	date := time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
	previousYear := date.AddDate(-1, 0, 0)
	return &PolygonDate{
		Year:  previousYear.Year(),
		Month: int(previousYear.Month()),
		Day:   previousYear.Day(),
	}
}

func NewPolygonDate(date string) (*PolygonDate, error) {
	var year, month, day int
	_, err := fmt.Sscanf(date, "%d-%d-%d", &year, &month, &day)
	if err != nil {
		return nil, fmt.Errorf("NewPolygonDate: %w", err)
	}

	return &PolygonDate{
		Year:  year,
		Month: month,
		Day:   day,
	}, nil
}
