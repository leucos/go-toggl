package resource

import "fmt"

type Type int

const (
	Clients Type = iota
	Projects
	Tags
	TimeEntries
)

var TypeMap = map[Type]string{
	Clients:     "clients",
	Projects:    "projects",
	Tags:        "tags",
	TimeEntries: "time_entries",
}

func (r Type) String() string {
	return TypeMap[r]
}

func GenerateUserResourceURL(Type Type) string {
	return fmt.Sprintf("/me/%s", Type)
}

func GenerateResourceURL(Type Type, wid int) string {
	return fmt.Sprintf("/workspaces/%d/"+Type.String(), wid)
}

func GenerateResourceURLWithID(Type Type, wid int, id int) string {
	return GenerateResourceURL(Type, wid) + fmt.Sprintf("/%d", id)
}
