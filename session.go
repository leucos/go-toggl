package toggl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// Session represents an active connection to the Toggl REST API.
type Session struct {
	APIToken string
	username string
	password string
	logger   *slog.Logger
}

// OpenSession opens a session using an existing API token.
func OpenSession(apiToken string) Session {
	return Session{
		APIToken: apiToken,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// NewSession creates a new session by retrieving a user's API token.
func NewSession(username, password string) (*Session, error) {
	session := Session{
		username: username,
		password: password,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	data, err := session.get(TogglAPI, "/me", nil)
	if err != nil {
		return nil, err
	}

	var account Account
	err = decodeAccount(data, &account)
	if err != nil {
		return nil, err
	}

	session.username = ""
	session.password = ""
	session.APIToken = account.APIToken

	return &session, nil
}

// DisableLog disables output to stderr
func (session *Session) DisableLog() {
	session.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

// EnableLog enables output to stderr
func (session *Session) EnableLog() {
	session.logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
}

// GetAccount returns a user's account information, including a list of active
// projects and timers.
func (session *Session) GetAccount() (Account, error) {
	params := map[string]string{"with_related_data": "true"}
	data, err := session.get(TogglAPI, "/me", params)
	if err != nil {
		return Account{}, fmt.Errorf("error getting session: %v", err)
	}

	var account Account
	err = decodeAccount(data, &account)
	if err != nil {
		return Account{}, fmt.Errorf("error decoding account data: %v", err)
	}

	return account, nil
}

// GetSummaryReport retrieves a summary report using Toggle's reporting API.
func (session *Session) GetSummaryReport(workspace int, since, until string) (SummaryReport, error) {
	params := map[string]string{
		"user_agent":   "jc-toggl",
		"grouping":     "projects",
		"since":        since,
		"until":        until,
		"rounding":     "on",
		"workspace_id": fmt.Sprintf("%d", workspace)}
	data, err := session.get(ReportsAPI, "/summary", params)
	if err != nil {
		return SummaryReport{}, err
	}
	session.logger.Debug("got data", "data", data)

	var report SummaryReport
	err = decodeSummaryReport(data, &report)
	return report, err
}

// GetDetailedReport retrieves a detailed report using Toggle's reporting API.
func (session *Session) GetDetailedReport(workspace int, since, until string, page int) (DetailedReport, error) {
	params := map[string]string{
		"user_agent":   "jc-toggl",
		"since":        since,
		"until":        until,
		"page":         fmt.Sprintf("%d", page),
		"rounding":     "on",
		"workspace_id": fmt.Sprintf("%d", workspace)}
	data, err := session.get(ReportsAPI, "/details", params)
	if err != nil {
		return DetailedReport{}, err
	}
	session.logger.Debug("got data", "data", data)

	var report DetailedReport
	err = decodeDetailedReport(data, &report)
	return report, err
}

// startTimeEntry unified way how to start new entries. Eventually it should replace StartTimeEntry and
// StartTimeEntryForProject functions, which are for time-being kept for compatibility.
func (session *Session) startTimeEntry(timeEntry timeEntryCreate) (TimeEntry, error) {
	return handleTimeEntryResponse(
		session.post(TogglAPI, generateResourceURL(timeEntries, timeEntry.WorkspaceId), timeEntry),
	)
}

// StartTimeEntry creates a new time entry.
func (session *Session) StartTimeEntry(description string, wid int) (TimeEntry, error) {
	return session.startTimeEntry(newStartEntryRequestData(description, wid))
}

// StartTimeEntryForProject creates a new time entry for a specific project. Note that the 'billable' option is only
// meaningful for Toggl Pro accounts; it will be ignored for free accounts.
func (session *Session) StartTimeEntryForProject(description string, wid int, projectID int, billable *bool) (TimeEntry, error) {
	entry := newStartEntryRequestData(description, wid)
	entry.ProjectID = &projectID

	if billable != nil {
		entry.Billable = *billable
	}

	return session.startTimeEntry(entry)
}

// GetCurrentTimeEntry returns the current time entry, that's running
func (session *Session) GetCurrentTimeEntry() (TimeEntry, error) {
	return handleTimeEntryResponse(
		session.get(TogglAPI, generateUserResourceURL(timeEntries)+"/current", nil),
	)
}

// GetTimeEntries returns a list of time entries
func (session *Session) GetTimeEntries(startDate, endDate time.Time) ([]TimeEntry, error) {
	data, err := session.get(
		TogglAPI,
		generateUserResourceURL(timeEntries),
		map[string]string{
			"start_date": startDate.Format(time.RFC3339),
			"end_date":   endDate.Format(time.RFC3339),
		},
	)

	if err != nil {
		return nil, err
	}

	var results []TimeEntry
	err = json.Unmarshal(data, &results)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// UpdateTimeEntry changes information about an existing time entry.
func (session *Session) UpdateTimeEntry(timer TimeEntry) (TimeEntry, error) {
	session.logger.Debug("updating timer", "timer", timer)
	return handleTimeEntryResponse(
		session.put(TogglAPI, generateResourceURLWithID(timeEntries, timer.Wid, timer.ID), timer),
	)
}

// ContinueTimeEntry continues a time entry, either by creating a new entry
// with the same description or by extending the duration of an existing entry.
// In both cases the new entry will have the same description and project ID as
// the existing one.
func (session *Session) ContinueTimeEntry(timer TimeEntry, duronly bool) (TimeEntry, error) {
	session.logger.Debug("continuing timer", "timer", timer)
	if duronly &&
		time.Now().Local().Format("2006-01-02") == timer.Start.Local().Format("2006-01-02") {
		// If we're doing a duration-only continuation for a timer today, then basically only unstop the timer
		return session.UnstopTimeEntry(timer)
	} else {
		// If we're not doing a duration-only continuation, or a duration timer
		// wasn't created today, start new time entry with same metadata
		entry := newStartEntryRequestData(timer.Description, timer.Wid)
		entry = entry.withMetadataFromTimeEntry(timer)

		return session.startTimeEntry(entry)
	}
}

// UnstopTimeEntry starts a new entry that is a copy of the given one, including
// the given timer's start time. The given time entry is then deleted.
func (session *Session) UnstopTimeEntry(timer TimeEntry) (TimeEntry, error) {
	session.logger.Debug("unstopping timer", "timer", timer)

	entry := newStartEntryRequestData(timer.Description, timer.Wid)
	entry = entry.withMetadataFromTimeEntry(timer)
	entry.Start = timer.Start

	newEntry, err := session.startTimeEntry(entry)
	if err != nil {
		return TimeEntry{}, err
	}
	if _, err = session.DeleteTimeEntry(timer); err != nil {
		err = fmt.Errorf("old entry not deleted: %v", err)
		return TimeEntry{}, err
	}

	return newEntry, nil
}

// StopTimeEntry stops a running time entry.
func (session *Session) StopTimeEntry(timer TimeEntry) (TimeEntry, error) {
	session.logger.Debug("stopping timer", "timer", timer)
	return handleTimeEntryResponse(
		session.patch(
			TogglAPI,
			generateResourceURLWithID(timeEntries, timer.Wid, timer.ID)+"/stop",
		),
	)
}

// AddRemoveTag adds or removes a tag from the time entry corresponding to a
// given ID.
func (session *Session) AddRemoveTag(timeEntryId int, tag string, add bool, wid int) (TimeEntry, error) {

	action := "add"
	if !add {
		action = "remove"
	}

	session.logger.Debug("changing tag in time entry", "action", action, "tag", tag, "timeEntryID", timeEntryId)

	data := map[string]interface{}{
		"tags":       []string{tag},
		"tag_action": action,
	}

	return handleTimeEntryResponse(
		session.put(TogglAPI, generateResourceURLWithID(timeEntries, wid, timeEntryId), data),
	)
}

// DeleteTimeEntry deletes a time entry.
func (session *Session) DeleteTimeEntry(timer TimeEntry) ([]byte, error) {
	session.logger.Debug("deleting timer", "timer", timer)
	return session.delete(TogglAPI, generateResourceURLWithID(timeEntries, timer.Wid, timer.ID))
}

// GetProjects allows to query for all projects in a workspace
func (session *Session) GetProjects(wid int) ([]Project, error) {
	session.logger.Debug("getting projects for workspace", "workspaceID", wid)
	data, err := session.get(TogglAPI, generateResourceURL(projects, wid), nil)
	if err != nil {
		return nil, err
	}

	var projects []Project
	err = json.Unmarshal(data, &projects)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

// GetProject allows to query for all projects in a workspace
func (session *Session) GetProject(id int, wid int) (project Project, err error) {
	session.logger.Debug("getting project", "projectID", id)
	data, err := session.get(TogglAPI, generateResourceURLWithID(projects, wid, id), nil)
	if err != nil {
		return project, err
	}

	err = json.Unmarshal(data, &project)
	if err != nil {
		return project, err
	}

	return project, nil
}

// CreateProject creates a new project.
func (session *Session) CreateProject(name string, wid int) (project Project, err error) {
	session.logger.Debug("creating project", "projectName", name)
	data := map[string]interface{}{
		"name":   name,
		"wid":    wid,
		"active": true,
	}

	respData, err := session.post(TogglAPI, generateResourceURL(projects, wid), data)
	if err != nil {
		return project, err
	}

	err = json.Unmarshal(respData, &project)
	if err != nil {
		return project, err
	}

	return project, nil
}

// UpdateProject changes information about an existing project.
func (session *Session) UpdateProject(project Project) (Project, error) {
	session.logger.Debug("updating project", "project", project)
	respData, err := session.put(
		TogglAPI,
		generateResourceURLWithID(projects, project.Wid, project.ID),
		project,
	)

	if err != nil {
		return Project{}, err
	}

	var entry Project
	err = json.Unmarshal(respData, &entry)
	if err != nil {
		return Project{}, err
	}

	return entry, nil
}

// DeleteProject deletes a project.
func (session *Session) DeleteProject(project Project) ([]byte, error) {
	session.logger.Debug("deleting project", "project", project)
	return session.delete(TogglAPI, generateResourceURLWithID(projects, project.Wid, project.ID))
}

// CreateTag creates a new tag.
func (session *Session) CreateTag(name string, wid int) (tag Tag, err error) {
	session.logger.Debug("Creating tag %s", name)
	data := map[string]interface{}{
		"name": name,
		"wid":  wid,
	}

	respData, err := session.post(TogglAPI, generateResourceURL(tags, wid), data)
	if err != nil {
		return tag, err
	}

	err = json.Unmarshal(respData, &tag)
	if err != nil {
		return tag, err
	}

	return tag, nil
}

// UpdateTag changes information about an existing tag.
func (session *Session) UpdateTag(tag Tag) (Tag, error) {
	session.logger.Debug("updating tag", "tag", tag)
	respData, err := session.put(TogglAPI, generateResourceURLWithID(tags, tag.Wid, tag.ID), tag)

	if err != nil {
		return Tag{}, err
	}

	var entry Tag
	err = json.Unmarshal(respData, &entry)
	if err != nil {
		return Tag{}, err
	}

	return entry, nil
}

// DeleteTag deletes a tag.
func (session *Session) DeleteTag(tag Tag) ([]byte, error) {
	session.logger.Debug("deleting tag", "tag", tag)
	return session.delete(TogglAPI, generateResourceURLWithID(tags, tag.Wid, tag.ID))
}

// GetClients returns a list of clients for the current account
func (session *Session) GetClients(wid int) (list []Client, err error) {
	session.logger.Debug("retrieving clients")

	data, err := session.get(TogglAPI, generateResourceURL(clients, wid), nil)
	if err != nil {
		return list, err
	}
	err = json.Unmarshal(data, &list)
	return list, err
}

// CreateClient adds a new client
func (session *Session) CreateClient(name string, wid int) (client Client, err error) {
	session.logger.Debug("creating client", "clientName", name)
	data := map[string]interface{}{
		"name": name,
		"wid":  wid,
	}

	respData, err := session.post(TogglAPI, generateResourceURL(clients, wid), data)
	if err != nil {
		return client, err
	}

	err = json.Unmarshal(respData, &client)
	if err != nil {
		return client, err
	}
	return client, nil
}

func (session *Session) request(method string, requestURL string, body io.Reader) ([]byte, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10

	client := retryClient.StandardClient() // *http.Client

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}

	if session.APIToken != "" {
		req.SetBasicAuth(session.APIToken, "api_token")
	} else {
		req.SetBasicAuth(session.username, session.password)
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return content, fmt.Errorf("response error: %s", resp.Status)
	}

	return content, nil
}

func (session *Session) get(requestURL string, path string, params map[string]string) ([]byte, error) {
	requestURL += path

	if params != nil {
		data := url.Values{}
		for key, value := range params {
			data.Set(key, value)
		}
		requestURL += "?" + data.Encode()
	}

	session.logger.Debug("GETing from URL: %s", requestURL)
	return session.request("GET", requestURL, nil)
}

func (session *Session) post(requestURL string, path string, data interface{}) ([]byte, error) {
	requestURL += path
	var body []byte
	var err error

	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	session.logger.Debug("POSTing to URL", "url", requestURL)
	session.logger.Debug("data", "data", body)
	return session.request("POST", requestURL, bytes.NewBuffer(body))
}

func (session *Session) put(requestURL string, path string, data interface{}) ([]byte, error) {
	requestURL += path
	var body []byte
	var err error

	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	session.logger.Debug("PUTing URL", "url", requestURL, "body", string(body))
	return session.request("PUT", requestURL, bytes.NewBuffer(body))
}

func (session *Session) patch(requestURL string, path string) ([]byte, error) {
	requestURL += path
	session.logger.Debug("PATCHing URL", "url", requestURL)
	return session.request("PATCH", requestURL, nil)
}

func (session *Session) delete(requestURL string, path string) ([]byte, error) {
	requestURL += path
	session.logger.Debug("DELETEing URL", "url", requestURL)
	return session.request("DELETE", requestURL, nil)
}

// func decodeSession(data []byte, session *Session) error {
// 	dec := json.NewDecoder(bytes.NewReader(data))
// 	err := dec.Decode(session)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
