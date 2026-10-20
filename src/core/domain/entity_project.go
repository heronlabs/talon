package domain

import (
	"slices"
	"strings"
	"time"

	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
)

// projectSeparator is the separator of the project in the name of a Swarm service.
const projectSeparator = "_"

// Project holds an Easypanel project: its name, its reconcile and its services.
type Project struct {
	Name      string     `json:"name"`
	Reconcile *Reconcile `json:"reconcile"`
	Services  []Service  `json:"services"`
}

// NewProjects returns each Swarm service of snapshot named <project>_<service>, split on the first _, under its project, in the order the projects first appear, with the bytes of the directory <project>/<service> of easypanelSnapshot and its recent failures counted at measuredAt, then each client of declared without a Swarm service, in the order of the manifest; the services without _ are skipped. Each project ends with the services its client declares without a Swarm service, in the order of its schema, and has its reconcile against configured.
func NewProjects(snapshot docker.Snapshot, easypanelSnapshot easypanel.Snapshot, declared *manifest.Manifest, configured *easypanel.ConfigSnapshot, measuredAt time.Time) []Project {
	clients := manifestClients(declared)
	projects := []Project{}
	for _, service := range snapshot.Services {
		if projectName, serviceName, found := strings.Cut(service.Spec.Name, projectSeparator); found {
			projects = addService(projects, projectName, NewService(serviceName, service, snapshot, easypanelSnapshot.Sizes[projectName][serviceName], findClient(clients, projectName), measuredAt))
		}
	}

	for _, client := range clients {
		projects = addProject(projects, client.Slug)
	}

	for index := range projects {
		projects[index].reconcile(findClient(clients, projects[index].Name), configured)
	}

	return projects
}

// reconcile appends to the services of the project each service that client declares without a Swarm service, in the order of its schema, and sets the reconcile of the project against configured.
func (p *Project) reconcile(client *manifest.Client, configured *easypanel.ConfigSnapshot) {
	for _, name := range declaredNames(client) {
		if !slices.ContainsFunc(p.Services, func(service Service) bool { return service.Name == name }) {
			p.Services = append(p.Services, NewMissingService(name))
		}
	}

	p.Reconcile = NewReconcile(p.Name, client, configured)
}

// manifestClients returns the clients of declared, none without it.
func manifestClients(declared *manifest.Manifest) []manifest.Client {
	if declared == nil {
		return nil
	}

	return declared.Data.Clients
}

// findClient returns the client of clients whose slug is name, nil when there is none.
func findClient(clients []manifest.Client, name string) *manifest.Client {
	index := slices.IndexFunc(clients, func(client manifest.Client) bool { return client.Slug == name })
	if index < 0 {
		return nil
	}

	return &clients[index]
}

// addProject returns projects with the project named name, without services, appended when it is not in projects yet.
func addProject(projects []Project, name string) []Project {
	if slices.ContainsFunc(projects, func(project Project) bool { return project.Name == name }) {
		return projects
	}

	return append(projects, Project{Name: name, Services: []Service{}})
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
