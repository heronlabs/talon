package domain

import (
	"path"
	"slices"
	"strings"
)

// EasypanelProjectsDir is where Easypanel keeps the bind-mounted data of each service.
const EasypanelProjectsDir = "/etc/easypanel/projects"

// BindMountPaths lists, without repeats and sorted, the bind sources of the containers of
// the Swarm service fullName that live under that service's Easypanel directory.
func BindMountPaths(d DockerSample, fullName string) []string {
	project, service, ok := SplitServiceName(fullName)
	if !ok {
		return nil
	}
	prefix := path.Join(EasypanelProjectsDir, project, service) + "/"
	var paths []string
	for _, c := range d.Containers {
		if c.ServiceName != fullName {
			continue
		}
		for _, src := range c.BindSources {
			clean := path.Clean(src)
			if strings.HasPrefix(clean+"/", prefix) && !slices.Contains(paths, clean) {
				paths = append(paths, clean)
			}
		}
	}
	slices.Sort(paths)
	return paths
}

// AllBindMountPaths lists the bind paths of every project service in the sample.
func AllBindMountPaths(d DockerSample) []string {
	var paths []string
	for _, svc := range d.Services {
		paths = append(paths, BindMountPaths(d, svc.Name)...)
	}
	return paths
}
