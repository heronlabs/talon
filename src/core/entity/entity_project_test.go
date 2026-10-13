package entity

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
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

		projects := NewProjects(snapshot, easypanel.Snapshot{}, nil, nil)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should split the service name on the first _", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"shop_api_v2"}}]`)}
		want := []Project{{Name: "shop", Services: []Service{{Name: "api_v2", State: "missing"}}}}

		projects := NewProjects(snapshot, easypanel.Snapshot{}, nil, nil)

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

		projects := NewProjects(snapshot, easypanel.Snapshot{}, nil, nil)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should give each service the bytes of the directory of its project and service", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[
			{"ID":"s1","Spec":{"Name":"shop_api"}},
			{"ID":"s2","Spec":{"Name":"blog_api"}},
			{"ID":"s3","Spec":{"Name":"blog_web"}}
		]`)}
		easypanelSnapshot := easypanel.Snapshot{Sizes: map[string]map[string]int64{
			"shop": {"api": 1000000000},
			"blog": {"api": 2000000000, "web": 500000000},
		}}
		want := []Project{
			{Name: "shop", Services: []Service{{Name: "api", State: "missing", DiskGB: 1}}},
			{Name: "blog", Services: []Service{{Name: "api", State: "missing", DiskGB: 2}, {Name: "web", State: "missing", DiskGB: 0.5}}},
		}

		projects := NewProjects(snapshot, easypanelSnapshot, nil, nil)

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

		projects := NewProjects(snapshot, easypanel.Snapshot{}, nil, nil)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should add each client of the manifest without a Swarm service after the Swarm projects, in the order of the manifest", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"shop_api"}}]`)}
		declared := decode[manifest.Manifest](t, `{"data":{"clients":[
			{"slug":"cms","schema":{"services":[]}},
			{"slug":"shop","schema":{"services":[]}},
			{"slug":"blog","schema":{"services":[]}}
		]}}`)
		want := []Project{
			{Name: "shop", Services: []Service{{Name: "api", State: "missing"}}},
			{Name: "cms", Services: []Service{}},
			{Name: "blog", Services: []Service{}},
		}

		projects := NewProjects(snapshot, easypanel.Snapshot{}, &declared, nil)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should add each service a client declares without a Swarm service after its Swarm services, in the order of its schema", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"shop_api"}}]`)}
		declared := decode[manifest.Manifest](t, `{"data":{"clients":[{"slug":"shop","schema":{"services":[
			{"type":"postgres","data":{"serviceName":"db"}},
			{"type":"app","data":{"serviceName":"api"}},
			{"type":"app","data":{"serviceName":"worker"}}
		]}}]}}`)
		want := []Project{{Name: "shop", Services: []Service{
			{Name: "api", Managed: true, State: "missing"},
			{Name: "db", Managed: true, State: "missing"},
			{Name: "worker", Managed: true, State: "missing"},
		}}}

		projects := NewProjects(snapshot, easypanel.Snapshot{}, &declared, nil)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should mark managed only the Swarm services their client declares", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[
			{"ID":"s1","Spec":{"Name":"shop_api"}},
			{"ID":"s2","Spec":{"Name":"shop_cron"}},
			{"ID":"s3","Spec":{"Name":"blog_api"}}
		]`)}
		declared := decode[manifest.Manifest](t, `{"data":{"clients":[{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api"}}]}}]}}`)
		want := []Project{
			{Name: "shop", Services: []Service{{Name: "api", Managed: true, State: "missing"}, {Name: "cron", State: "missing"}}},
			{Name: "blog", Services: []Service{{Name: "api", State: "missing"}}},
		}

		projects := NewProjects(snapshot, easypanel.Snapshot{}, &declared, nil)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should give each project its reconcile against the configuration of Easypanel", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"blog_web"}}]`)}
		declared := decode[manifest.Manifest](t, `{"data":{"clients":[
			{"slug":"shop","schema":{"services":[{"type":"postgres","data":{"serviceName":"db","image":"postgres:18"}}]}},
			{"slug":"cms","schema":{"services":[{"type":"postgres","data":{"serviceName":"db","image":"postgres:18"}}]}}
		]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{
			"shop": {"db": {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:18"}`)}},
			"cms":  {"db": {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:17"}`)}},
		}}
		want := []string{"unmanaged", "converged", "pending"}

		projects := NewProjects(snapshot, easypanel.Snapshot{}, &declared, configured)
		states := []string{projects[0].Reconcile.State, projects[1].Reconcile.State, projects[2].Reconcile.State}

		if !reflect.DeepEqual(states, want) {
			t.Errorf("NewProjects() reconcile states = %v, want %v", states, want)
		}
	})

	t.Run("Should leave the reconcile of every project null without the configuration of Easypanel", func(t *testing.T) {
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"blog_web"}}]`)}
		declared := decode[manifest.Manifest](t, `{"data":{"clients":[{"slug":"shop","schema":{"services":[]}}]}}`)
		want := []Project{
			{Name: "blog", Services: []Service{{Name: "web", State: "missing"}}},
			{Name: "shop", Services: []Service{}},
		}

		projects := NewProjects(snapshot, easypanel.Snapshot{}, &declared, nil)

		if !reflect.DeepEqual(projects, want) {
			t.Errorf("NewProjects() = %+v, want %+v", projects, want)
		}
	})

	t.Run("Should encode the services of a client without any as []", func(t *testing.T) {
		declared := decode[manifest.Manifest](t, `{"data":{"clients":[{"slug":"shop","schema":{"services":[]}}]}}`)

		projects := NewProjects(docker.Snapshot{}, easypanel.Snapshot{}, &declared, nil)
		encoded, _ := json.Marshal(projects)

		if !strings.Contains(string(encoded), `"services":[]`) {
			t.Errorf("NewProjects() encoded %s, want %s", encoded, `"services":[]`)
		}
	})
}
