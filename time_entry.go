package toggl

import (
	"encoding/json"
	"fmt"
	"time"
)

// Tag represents a tag.
type Tag struct {
	Wid  int    `json:"workspace_id"`
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TimeEntry represents a single time entry.
type TimeEntry struct {
	Wid         int        `json:"workspace_id,omitempty"`
	ID          int        `json:"id,omitempty"`
	Pid         *int       `json:"project_id,omitempty"`
	Tid         *int       `json:"task_id,omitempty"`
	Description string     `json:"description,omitempty"`
	Stop        *time.Time `json:"stop,omitempty"`
	Start       *time.Time `json:"start,omitempty"`
	Tags        []string   `json:"tags"`
	Duration    int64      `json:"duration,omitempty"`
	DurOnly     bool       `json:"duronly"`
	Billable    bool       `json:"billable"`
}

type DetailedTimeEntry struct {
	ID              int        `json:"id"`
	Pid             int        `json:"pid"`
	Tid             int        `json:"tid"`
	Uid             int        `json:"uid"`
	User            string     `json:"user,omitempty"`
	Description     string     `json:"description"`
	Project         string     `json:"project"`
	ProjectColor    string     `json:"project_color"`
	ProjectHexColor string     `json:"project_hex_color"`
	Client          string     `json:"client"`
	Start           *time.Time `json:"start"`
	End             *time.Time `json:"end"`
	Updated         *time.Time `json:"updated"`
	Duration        int64      `json:"dur"`
	Billable        bool       `json:"billable"`
	Tags            []string   `json:"tags"`
}

type timeEntryCreate struct {
	Billable    bool       `json:"billable"`
	Description string     `json:"description"`
	Duration    int        `json:"duration"`
	ProjectID   *int       `json:"project_id,omitempty"`
	TaskID      *int       `json:"task_id,omitempty"`
	Start       *time.Time `json:"start,omitempty"`
	Stop        *time.Time `json:"stop,omitempty"`
	Tags        []string   `json:"tags"`
	WorkspaceId int        `json:"workspace_id"`
}

// This is an alias for TimeEntry that is used in tempTimeEntry to prevent the
// unmarshaler from infinitely recursing while unmarshaling.
type embeddedTimeEntry TimeEntry

// tempTimeEntry is an intermediate type used as for decoding TimeEntries.
type tempTimeEntry struct {
	embeddedTimeEntry
	Stop  string `json:"stop"`
	Start string `json:"start"`
}

func (t *tempTimeEntry) asTimeEntry() (entry TimeEntry, err error) {
	entry = TimeEntry(t.embeddedTimeEntry)

	parseTime := func(s string) (t time.Time, err error) {
		t, err = time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05-07:00", s)
		}
		return
	}

	if t.Start != "" {
		var start time.Time
		start, err = parseTime(t.Start)
		if err != nil {
			return
		}
		entry.Start = &start
	}

	if t.Stop != "" {
		var stop time.Time
		stop, err = parseTime(t.Stop)
		if err != nil {
			return
		}
		entry.Stop = &stop
	}

	return
}

func handleTimeEntryResponse(data []byte, err error) (TimeEntry, error) {
	if err != nil {
		return TimeEntry{}, err
	}

	var entry TimeEntry
	err = json.Unmarshal(data, &entry)
	dlog.Printf("Unmarshaled '%s' into %#v\n", data, entry)
	if err != nil {
		return TimeEntry{}, err
	}

	return entry, nil
}

func (t timeEntryCreate) MarshalJSON() ([]byte, error) {
	type Alias timeEntryCreate
	return json.Marshal(&struct {
		Alias
		CreatedWith string `json:"created_with"`
	}{
		Alias:       (Alias)(t),
		CreatedWith: AppName,
	})
}

func (t timeEntryCreate) withMetadataFromTimeEntry(timeEntry TimeEntry) timeEntryCreate {
	t.ProjectID = timeEntry.Pid
	t.TaskID = timeEntry.Tid
	t.Tags = timeEntry.Tags
	t.Billable = timeEntry.Billable

	return t
}

func newStartEntryRequestData(description string, workspaceId int) timeEntryCreate {
	now := time.Now()
	return timeEntryCreate{
		Duration:    -1,
		Description: description,
		Start:       &now,
		WorkspaceId: workspaceId,
	}
}

// IsRunning returns true if the receiver is currently running.
func (e *TimeEntry) IsRunning() bool {
	return e.Duration < 0
}

// Copy returns a copy of a TimeEntry.
func (e *TimeEntry) Copy() TimeEntry {
	newEntry := *e
	newEntry.Tags = make([]string, len(e.Tags))
	copy(newEntry.Tags, e.Tags)
	if e.Start != nil {
		*newEntry.Start = *e.Start
	}
	if e.Stop != nil {
		*newEntry.Stop = *e.Stop
	}
	return newEntry
}

// StartTime returns the start time of a time entry as a time.Time.
func (e *TimeEntry) StartTime() time.Time {
	if e.Start != nil {
		return *e.Start
	}
	return time.Time{}
}

// StopTime returns the stop time of a time entry as a time.Time.
func (e *TimeEntry) StopTime() time.Time {
	if e.Stop != nil {
		return *e.Stop
	}
	return time.Time{}
}

// HasTag returns true if a time entry contains a given tag.
func (e *TimeEntry) HasTag(tag string) bool {
	return indexOfTag(tag, e.Tags) != -1
}

// AddTag adds a tag to a time entry if the entry doesn't already contain the
// tag.
func (e *TimeEntry) AddTag(tag string) {
	if !e.HasTag(tag) {
		e.Tags = append(e.Tags, tag)
	}
}

// RemoveTag removes a tag from a time entry.
func (e *TimeEntry) RemoveTag(tag string) {
	if i := indexOfTag(tag, e.Tags); i != -1 {
		e.Tags = append(e.Tags[:i], e.Tags[i+1:]...)
	}
}

// SetDuration sets a time entry's duration. The duration should be a value in
// seconds. The stop time will also be updated. Note that the time entry must
// not be running.
func (e *TimeEntry) SetDuration(duration int64) error {
	if e.IsRunning() {
		return fmt.Errorf("TimeEntry must be stopped")
	}

	e.Duration = duration
	newStop := e.Start.Add(time.Duration(duration) * time.Second)
	e.Stop = &newStop

	return nil
}

// SetStartTime sets a time entry's start time. If the time entry is stopped,
// the stop time will also be updated.
func (e *TimeEntry) SetStartTime(start time.Time, updateEnd bool) {
	e.Start = &start

	if !e.IsRunning() {
		if updateEnd {
			newStop := start.Add(time.Duration(e.Duration) * time.Second)
			e.Stop = &newStop
		} else {
			e.Duration = e.Stop.Unix() - e.Start.Unix()
		}
	}
}

// SetStopTime sets a time entry's stop time. The duration will also be
// updated. Note that the time entry must not be running.
func (e *TimeEntry) SetStopTime(stop time.Time) (err error) {
	if e.IsRunning() {
		return fmt.Errorf("TimeEntry must be stopped")
	}

	e.Stop = &stop
	e.Duration = int64(stop.Sub(*e.Start) / time.Second)

	return nil
}

func indexOfTag(tag string, tags []string) int {
	for i, t := range tags {
		if t == tag {
			return i
		}
	}
	return -1
}

// UnmarshalJSON unmarshals a TimeEntry from JSON data, converting timestamp
// fields to Go Time values.
func (e *TimeEntry) UnmarshalJSON(b []byte) error {
	var entry tempTimeEntry
	err := json.Unmarshal(b, &entry)
	if err != nil {
		return err
	}
	te, err := entry.asTimeEntry()
	if err != nil {
		return err
	}
	*e = te
	return nil
}
