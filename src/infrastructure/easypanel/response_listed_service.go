package easypanel

// ListedService is a service in the answer of listProjectsAndServices: its name, its project and its type.
type ListedService struct {
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
	Type        string `json:"type"`
}
