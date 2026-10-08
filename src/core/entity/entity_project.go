package entity

import (
	"slices"
	"strings"

	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
)

// projectSeparator is the separator of the project in the name of a Swarm service.
const projectSeparator = "_"

// Project holds an Easypanel project: its name, its reconcile and its services.
type Project struct {
	Name      string     `json:"name"`
	Reconcile *Reconcile `json:"reconcile"`
	Services  []Service  `json:"services"`
}

// NewProjects returns each Swarm service of snapshot named <project>_<service>, split on the first _, under its project, in the order the projects first appear, with the bytes of the directory <project>/<service> of easypanelSnapshot; the services without _ are skipped.
func NewProjects(snapshot docker.Snapshot, easypanelSnapshot easypanel.Snapshot) []Project {
	projects := []Project{}
	for _, service := range snapshot.Services {
		if projectName, serviceName, found := strings.Cut(service.Spec.Name, projectSeparator); found {
			projects = addService(projects, projectName, NewService(serviceName, service, snapshot, easypanelSnapshot.Sizes[projectName][serviceName]))
		}
	}

	return projects
}

// addService returns projects with service under the project named name, appending that project when it is not in projects yet.
func addService(projects []Project, name string, service Service) []Project {
	index := slices.IndexFunc(projects, func(project Project) bool { return project.Name == name })
	if index < 0 {
		return append(projects, Project{Name: name, Services: []Service{service}})
	}

	projects[index].Services = append(projects[index].Services, service)

	return projects
}
