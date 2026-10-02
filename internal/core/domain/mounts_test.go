package domain_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

type split struct {
	Project, Service string
	OK               bool
}

func splitOf(full string) split {
	p, s, ok := domain.SplitServiceName(full)
	return split{p, s, ok}
}

func TestSplitServiceName_CutsAtFirstUnderscore(t *testing.T) {
	got := splitOf("cms-heronlabs_cms_api")

	testkit.Equal(t, split{"cms-heronlabs", "cms_api", true}, got)
}

func TestSplitServiceName_RejectsNameWithoutUnderscore(t *testing.T) {
	got := splitOf("easypanel")

	testkit.Equal(t, split{"easypanel", "", false}, got)
}

func TestSplitServiceName_RejectsEmptyProject(t *testing.T) {
	got := splitOf("_api")

	testkit.Equal(t, split{"", "api", false}, got)
}

func TestSplitServiceName_RejectsEmptyService(t *testing.T) {
	got := splitOf("p_")

	testkit.Equal(t, split{"p", "", false}, got)
}

func TestBindMountPaths_KeepsTheServiceDirsSortedWithoutRepeats(t *testing.T) {
	d := domain.DockerSample{Containers: []domain.Container{
		{ServiceName: "p_db", BindSources: []string{
			"/etc/easypanel/projects/p/db/data/",
			"/etc/easypanel/projects/p/db/conf",
			"/etc/easypanel/projects/p/dbx/conf",
			"/etc/easypanel/projects/p/api/conf",
			"/var/run/docker.sock",
		}},
		{ServiceName: "p_db", BindSources: []string{"/etc/easypanel/projects/p/db/data"}},
		{ServiceName: "p_api", BindSources: []string{"/etc/easypanel/projects/p/db/other"}},
	}}

	got := domain.BindMountPaths(d, "p_db")

	testkit.Equal(t, []string{"/etc/easypanel/projects/p/db/conf", "/etc/easypanel/projects/p/db/data"}, got)
}

func TestBindMountPaths_ServiceDirItselfCounts(t *testing.T) {
	d := domain.DockerSample{Containers: []domain.Container{
		{ServiceName: "p_db", BindSources: []string{"/etc/easypanel/projects/p/db"}},
	}}

	got := domain.BindMountPaths(d, "p_db")

	testkit.Equal(t, []string{"/etc/easypanel/projects/p/db"}, got)
}

func TestBindMountPaths_NameWithoutProjectGivesNothing(t *testing.T) {
	d := domain.DockerSample{Containers: []domain.Container{
		{ServiceName: "easypanel", BindSources: []string{"/etc/easypanel"}},
	}}

	got := domain.BindMountPaths(d, "easypanel")

	testkit.Equal(t, []string(nil), got)
}

func TestAllBindMountPaths_JoinsEveryService(t *testing.T) {
	d := domain.DockerSample{
		Services: []domain.SwarmService{{Name: "p_api"}, {Name: "p_db"}, {Name: "easypanel"}},
		Containers: []domain.Container{
			{ServiceName: "p_api", BindSources: []string{"/etc/easypanel/projects/p/api/a"}},
			{ServiceName: "p_db", BindSources: []string{"/etc/easypanel/projects/p/db/b"}},
		},
	}

	got := domain.AllBindMountPaths(d)

	testkit.Equal(t, []string{"/etc/easypanel/projects/p/api/a", "/etc/easypanel/projects/p/db/b"}, got)
}
