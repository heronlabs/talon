package environment

import (
	"slices"
	"testing"
)

func TestConfig(t *testing.T) {
	t.Run("Should list every key in the order install asks for them", func(t *testing.T) {
		want := []Key{
			{Name: "CMS_URL"},
			{Name: "CMS_AGENT_TOKEN"},
			{Name: "CMS_AGENT_HOST_NAME"},
			{Name: "EASYPANEL_URL", Default: "http://127.0.0.1:3000"},
			{Name: "EASYPANEL_TOKEN"},
			{Name: "AWS_ACCESS_KEY_ID"},
			{Name: "AWS_SECRET_ACCESS_KEY"},
			{Name: "AWS_REGION", Default: "us-east-1"},
			{Name: "INTERVAL_RECONCILE", Default: "5m"},
			{Name: "INTERVAL_REPORT", Default: "10m"},
			{Name: "DOCKER_SOCKET", Default: "/var/run/docker.sock"},
		}

		keys := Keys()

		if !slices.Equal(keys, want) {
			t.Errorf("Keys() = %v, want %v", keys, want)
		}
	})

	t.Run("Should require a key without default", func(t *testing.T) {
		key := Key{Name: "CMS_URL"}

		required := key.Required()

		if !required {
			t.Errorf("Required() = %v, want %v", required, true)
		}
	})

	t.Run("Should not require a key with default", func(t *testing.T) {
		key := Key{Name: "AWS_REGION", Default: "us-east-1"}

		required := key.Required()

		if required {
			t.Errorf("Required() = %v, want %v", required, false)
		}
	})
}
