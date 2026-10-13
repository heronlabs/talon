package entity

import (
	"reflect"
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
)

func TestReconcile(t *testing.T) {
	t.Run("Should be nil without the configuration of Easypanel", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[]}}`)

		reconcile := NewReconcile("shop", &client, nil)

		if reconcile != nil {
			t.Errorf("NewReconcile() = %+v, want nil", reconcile)
		}
	})

	t.Run("Should be unmanaged when the project is not in the manifest", func(t *testing.T) {
		configured := &easypanel.ConfigSnapshot{}

		reconcile := NewReconcile("shop", nil, configured)

		if reconcile == nil || reconcile.State != "unmanaged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "unmanaged")
		}
	})

	t.Run("Should be converged with no missing secret and no error when every declared service matches its configured one", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[
			{"type":"app","data":{"projectName":"shop","serviceName":"api","source":{"type":"image","image":"nginx:latest"}}},
			{"type":"postgres","data":{"projectName":"shop","serviceName":"db","image":"postgres:18"}}
		]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"source":{"type":"image","image":"nginx:latest"}}`)},
			"db":  {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:18"}`)},
		}}}
		want := &Reconcile{State: "converged", MissingSecrets: []string{}}

		reconcile := NewReconcile("shop", &client, configured)

		if !reflect.DeepEqual(reconcile, want) {
			t.Errorf("NewReconcile() = %+v, want %+v", reconcile, want)
		}
	})

	t.Run("Should be pending when the client declares no service", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","lifecycle":"provisioning","secretsRoleArn":null}`)
		configured := &easypanel.ConfigSnapshot{}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should be pending when a declared service is not configured", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"postgres","data":{"serviceName":"db","image":"postgres:18"}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"blog": {
			"db": {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:18"}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should be pending when a declared service is configured with another type", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"mysql","data":{"serviceName":"db","image":"postgres:18"}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"db": {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:18"}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should be pending when one of the declared services does not match", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[
			{"type":"postgres","data":{"serviceName":"db","image":"postgres:18"}},
			{"type":"app","data":{"serviceName":"api","deploy":{"replicas":2}}}
		]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"db":  {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:18"}`)},
			"api": {Type: "app", Data: decode[map[string]any](t, `{"deploy":{"replicas":1}}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should be pending when a declared scalar differs", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"postgres","data":{"serviceName":"db","image":"postgres:17"}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"db": {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:18"}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should be pending when a declared key is not configured", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"postgres","data":{"serviceName":"db","image":"postgres:18","user":"postgres"}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"db": {Type: "postgres", Data: decode[map[string]any](t, `{"image":"postgres:18"}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should ignore the configured keys the schema does not declare, at any depth", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","deploy":{"replicas":1}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"enabled":true,"deploy":{"replicas":1,"zeroDowntime":true},"mounts":[]}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})

	t.Run("Should be pending when a nested declared value differs", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","resources":{"memoryLimit":2048}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"resources":{"cpuLimit":1,"memoryLimit":1024}}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should be pending when a declared object is configured as another kind of value", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","source":{"type":"image"}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"source":"image"}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should compare numbers as numbers", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","resources":{"cpuLimit":1,"memoryLimit":1024.0}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"resources":{"cpuLimit":1.0,"memoryLimit":1024}}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})

	t.Run("Should match a declared null with a configured null", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","deploy":{"command":null}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"deploy":{"command":null}}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})

	t.Run("Should match a declared null with an absent key", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","deploy":{"command":null}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"deploy":{}}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})

	t.Run("Should be pending when a declared null is configured with a value", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","deploy":{"command":null}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"deploy":{"command":"npm start"}}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should match the items of an array in any order", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","domains":[{"host":"a.example.com"},{"host":"b.example.com"}]}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"domains":[{"host":"b.example.com","port":80},{"host":"a.example.com","port":80}]}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})

	t.Run("Should be pending when a declared array is shorter than the configured one", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","domains":[{"host":"a.example.com"}]}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"domains":[{"host":"a.example.com"},{"host":"b.example.com"}]}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should be pending when two declared items match only the same configured item", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","ports":[{"published":8080},{"published":8080}]}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"ports":[{"published":8080},{"published":9090}]}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should give each declared item a distinct configured item when the first match is needed by a later item", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","ports":[{"published":8080},{"published":8080,"target":80}]}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"ports":[{"published":8080,"target":80},{"published":8080,"target":81}]}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})

	t.Run("Should be pending when a declared array is configured as another kind of value", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","mounts":[]}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"mounts":null}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "pending" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "pending")
		}
	})

	t.Run("Should not compare projectName and serviceName", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"postgres","data":{"projectName":"shop","serviceName":"db","image":"postgres:18"}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"db": {Type: "postgres", Data: decode[map[string]any](t, `{"name":"db","image":"postgres:18"}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})

	t.Run("Should ignore env and password at any depth", func(t *testing.T) {
		client := decode[manifest.Client](t, `{"slug":"shop","schema":{"services":[{"type":"app","data":{"serviceName":"api","env":"KEY=value","source":{"type":"image","image":"nginx:latest","password":"secret"}}}]}}`)
		configured := &easypanel.ConfigSnapshot{Services: map[string]map[string]easypanel.ServiceConfig{"shop": {
			"api": {Type: "app", Data: decode[map[string]any](t, `{"source":{"type":"image","image":"nginx:latest"}}`)},
		}}}

		reconcile := NewReconcile("shop", &client, configured)

		if reconcile == nil || reconcile.State != "converged" {
			t.Errorf("NewReconcile() = %+v, want state %q", reconcile, "converged")
		}
	})
}
