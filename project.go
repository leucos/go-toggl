package toggl

import "time"

// Project represents a project.
type Project struct {
	Wid             int        `json:"workspace_id"`
	ID              int        `json:"id"`
	Cid             *int       `json:"client_id,omitempty"`
	Name            string     `json:"name"`
	Active          bool       `json:"active"`
	Billable        *bool      `json:"billable,omitempty"`
	ServerDeletedAt *time.Time `json:"server_deleted_at,omitempty"`
}

// IsActive indicates whether a project exists and is active
func (p *Project) IsActive() bool {
	return p.Active && p.ServerDeletedAt == nil
}
