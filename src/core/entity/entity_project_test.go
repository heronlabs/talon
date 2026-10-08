package entity

import (
	"reflect"
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

func TestProject(t *testing.T) {
	t.Run("Should group the services under their project", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[
			{"ID":"s1","Spec":{"Name":"shop_api"}},
			{"ID":"s2","Spec":{"Name":"blog_web"}},
			{"ID":"s3","Spec":{"Name":"shop_worker"}}
		]`)}
		want := []Project{
			{Name: "shop", Services: []Service{{Name: "api", State: "missing"}, {Name: "worker", State: "missing"}}},
			{Name: "blog", Services: []Service{{Name: "web", State: "missing"}}},
		}

		projects := NewProjects(snapshot)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should split the service name on the first _", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"shop_api_v2"}}]`)}
		want := []Project{{Name: "shop", Services: []Service{{Name: "api_v2", State: "missing"}}}}

		projects := NewProjects(snapshot)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should skip the services without _", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[
			{"ID":"s1","Spec":{"Name":"traefik"}},
			{"ID":"s2","Spec":{"Name":"shop_api"}}
		]`)}
		want := []Project{{Name: "shop", Services: []Service{{Name: "api", State: "missing"}}}}

		projects := NewProjects(snapshot)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should order the projects as they first appear", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[
			{"ID":"s1","Spec":{"Name":"shop_api"}},
			{"ID":"s2","Spec":{"Name":"blog_web"}}
		]`)}
		want := []Project{
			{Name: "shop", Services: []Service{{Name: "api", State: "missing"}}},
			{Name: "blog", Services: []Service{{Name: "web", State: "missing"}}},
		}

		projects := NewProjects(snapshot)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})
}
