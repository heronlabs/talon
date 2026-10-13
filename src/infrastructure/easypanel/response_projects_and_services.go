package easypanel

// ProjectsAndServices is the answer of listProjectsAndServices: every service of every project.
type ProjectsAndServices struct {
	Services []ListedService `json:"services"`
}
