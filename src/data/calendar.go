package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func FetchCalendar(startDate, endDate eventmodels.PolygonDate) ([]*eventmodels.Calendar, error) {
	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	if err != nil {
		return nil, fmt.Errorf("FetchCalendar: error getting PROJECTS_DIR: %w", err)
	}

	anacondaHome, err := utils.GetEnv("ANACONDA_HOME")
	if err != nil {
		return nil, fmt.Errorf("FetchCalendar: error getting ANACONDA_HOME: %w", err)
	}

	interpreter := path.Join(anacondaHome, "envs", "grodt", "bin", "python3")
	scriptDir := path.Join(projectsDir, "slack-trading", "src", "cmd", "pandas_market_calendars", "main.py")
	startDateArg := startDate.ToString()
	endDateArg := endDate.ToString()

	cmd := exec.Command(interpreter, scriptDir, startDateArg, endDateArg)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("FetchCalendar: error running main.py: %w", err)
	}

	// Unmarshal CSV data
	schedules, err := unmarshalCSV(output)
	if err != nil {
		return nil, fmt.Errorf("FetchCalendar: error unmarshalling CSV: %w", err)
	}

	var result []*eventmodels.Calendar
	for _, schedule := range schedules {
		result = append(result, &schedule)
	}

	return result, nil
}

func FetchCalendarMap(startDate, endDate eventmodels.PolygonDate) (map[string]*eventmodels.Calendar, error) {
	scheduleMap := make(map[string]*eventmodels.Calendar)

	schedules, err := FetchCalendar(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("FetchCalendarMap: error fetching calendar: %w", err)
	}

	for _, schedule := range schedules {
		scheduleMap[schedule.Date] = schedule
	}

	return scheduleMap, nil
}

func unmarshalCSV(data []byte) ([]eventmodels.Calendar, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.FieldsPerRecord = -1 // Allow variable number of fields per record

	// Read the header
	_, err := r.Read()
	if err != nil {
		return nil, err
	}

	var schedules []eventmodels.Calendar
	const layout = "2006-01-02 15:04:05-07:00" // Custom layout to match the time format

	for {
		record, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		marketOpen, err := time.Parse(layout, record[1])
		if err != nil {
			return nil, err
		}

		marketClose, err := time.Parse(layout, record[2])
		if err != nil {
			return nil, err
		}

		schedule := eventmodels.Calendar{
			Date:        record[0],
			MarketOpen:  marketOpen,
			MarketClose: marketClose,
		}
		schedules = append(schedules, schedule)
	}

	if len(schedules) == 0 {
		log.Warn("unmarshalCSV: no schedules found")
	}

	return schedules, nil
}
