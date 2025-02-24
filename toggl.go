/*
Package toggl provides an API for interacting with the Toggl time tracking service.

See https://github.com/toggl/toggl_api_docs for more information on Toggl's REST API.
*/
package toggl

import (
	"bytes"
	"encoding/json"
)

// Toggl service constants
const (
	TogglAPI       = "https://api.track.toggl.com/api/v9"
	ReportsAPI     = "https://api.track.toggl.com/reports/api/v2"
	DefaultAppName = "go-toggl"
)

var (
	// AppName is the application name used when creating timers.
	AppName = DefaultAppName
)

// Account represents a user account.
type Account struct {
	APIToken        string      `json:"api_token"`
	Timezone        string      `json:"timezone"`
	ID              int         `json:"id"`
	Workspaces      []Workspace `json:"workspaces"`
	Clients         []Client    `json:"clients"`
	Projects        []Project   `json:"projects"`
	Tasks           []Task      `json:"tasks"`
	Tags            []Tag       `json:"tags"`
	TimeEntries     []TimeEntry `json:"time_entries"`
	BeginningOfWeek int         `json:"beginning_of_week"`
}

// Task represents a task.
type Task struct {
	Wid  int    `json:"wid"`
	Pid  int    `json:"pid"`
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Client represents a client.
type Client struct {
	Wid      int    `json:"wid"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	Notes    string `json:"notes"`
}

// Workspace represents a user workspace.
type Workspace struct {
	ID              int    `json:"id"`
	RoundingMinutes int    `json:"rounding_minutes"`
	Rounding        int    `json:"rounding"`
	Name            string `json:"name"`
	Premium         bool   `json:"premium"`
}

// SummaryReport represents a summary report generated by Toggl's reporting API.
type SummaryReport struct {
	TotalGrand int `json:"total_grand"`
	Data       []struct {
		ID    int `json:"id"`
		Time  int `json:"time"`
		Title struct {
			Project  string `json:"project"`
			Client   string `json:"client"`
			Color    string `json:"color"`
			HexColor string `json:"hex_color"`
		} `json:"title"`
		Items []struct {
			Title map[string]string `json:"title"`
			Time  int               `json:"time"`
		} `json:"items"`
	} `json:"data"`
}

// DetailedReport represents a summary report generated by Toggl's reporting API.
type DetailedReport struct {
	TotalGrand int                 `json:"total_grand"`
	TotalCount int                 `json:"total_count"`
	PerPage    int                 `json:"per_page"`
	Data       []DetailedTimeEntry `json:"data"`
}

func decodeAccount(data []byte, account *Account) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(account)
	if err != nil {
		return err
	}
	return nil
}

func decodeSummaryReport(data []byte, report *SummaryReport) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&report)
	if err != nil {
		return err
	}
	return nil
}

func decodeDetailedReport(data []byte, report *DetailedReport) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&report)
	if err != nil {
		return err
	}
	return nil
}
