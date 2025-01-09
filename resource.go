package toggl

import "fmt"

type resourceType int

const (
	clients resourceType = iota
	projects
	tags
	timeEntries
)

var resourceTypeMap = map[resourceType]string{
	clients:     "clients",
	projects:    "projects",
	tags:        "tags",
	timeEntries: "time_entries",
}

func (r resourceType) String() string {
	return resourceTypeMap[r]
}

func generateUserResourceURL(resourceType resourceType) string {
	return fmt.Sprintf("/me/%s", resourceType)
}

func generateResourceURL(resourceType resourceType, wid int) string {
	return fmt.Sprintf("/workspaces/%d/"+resourceType.String(), wid)
}

func generateResourceURLWithID(resourceType resourceType, wid int, id int) string {
	return generateResourceURL(resourceType, wid) + fmt.Sprintf("/%d", id)
}
