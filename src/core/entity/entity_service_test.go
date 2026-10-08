package entity

import (
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

func TestService(t *testing.T) {
	t.Run("Should be missing when the service has no container", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		want := Service{Name: "api", State: "missing"}

		service := NewService("api", swarmService, docker.Snapshot{}, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should be running over restarting", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[
			{"Id":"a1","State":"restarting","Labels":{"com.docker.swarm.service.name":"shop_api"}},
			{"Id":"b2","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}}
		]`)}
		want := Service{Name: "api", State: "running"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should be restarting over paused", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[
			{"Id":"a1","State":"paused","Labels":{"com.docker.swarm.service.name":"shop_api"}},
			{"Id":"b2","State":"restarting","Labels":{"com.docker.swarm.service.name":"shop_api"}}
		]`)}
		want := Service{Name: "api", State: "restarting"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should be paused over created", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[
			{"Id":"a1","State":"created","Labels":{"com.docker.swarm.service.name":"shop_api"}},
			{"Id":"b2","State":"paused","Labels":{"com.docker.swarm.service.name":"shop_api"}}
		]`)}
		want := Service{Name: "api", State: "paused"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should be created over exited", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[
			{"Id":"a1","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"}},
			{"Id":"b2","State":"created","Labels":{"com.docker.swarm.service.name":"shop_api"}}
		]`)}
		want := Service{Name: "api", State: "created"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should be exited over dead", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[
			{"Id":"a1","State":"dead","Labels":{"com.docker.swarm.service.name":"shop_api"}},
			{"Id":"b2","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"}}
		]`)}
		want := Service{Name: "api", State: "exited"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should count a removing container as exited", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[{"Id":"a1","State":"removing","Labels":{"com.docker.swarm.service.name":"shop_api"}}]`)}
		want := Service{Name: "api", State: "exited"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should be dead when no container state is known", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[{"Id":"a1","State":"unknown","Labels":{"com.docker.swarm.service.name":"shop_api"}}]`)}
		want := Service{Name: "api", State: "dead"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should match the containers to the service by the service name label", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Containers: decode[[]docker.Container](t, `[
			{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_web"}},
			{"Id":"b2","State":"paused","Labels":{"com.docker.swarm.service.name":"shop_api"}}
		]`)}
		want := Service{Name: "api", State: "paused"}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should report the replicas running and desired", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"},"ServiceStatus":{"RunningTasks":2,"DesiredTasks":3}}`)
		want := Service{Name: "api", State: "missing", ReplicasRunning: 2, ReplicasDesired: 3}

		service := NewService("api", swarmService, docker.Snapshot{}, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should count the failed or rejected tasks of the service", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{Tasks: decode[[]docker.SwarmTask](t, `[
			{"ServiceID":"s1","Status":{"State":"failed"}},
			{"ServiceID":"s1","Status":{"State":"rejected"}},
			{"ServiceID":"s1","Status":{"State":"running"}},
			{"ServiceID":"s1","Status":{"State":"shutdown"}},
			{"ServiceID":"s2","Status":{"State":"failed"}}
		]`)}
		want := Service{Name: "api", State: "missing", RecentTaskFailures: 2}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should report the memory limit in MiB and the CPU limit in CPUs", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api","TaskTemplate":{"Resources":{"Limits":{"NanoCPUs":1500000000,"MemoryBytes":536870912}}}}}`)
		want := Service{Name: "api", State: "missing", MemoryLimitMiB: 512, CPULimit: 1.5}

		service := NewService("api", swarmService, docker.Snapshot{}, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should report the size of the named volumes in GB, each counted once", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{
			DiskUsage: decode[docker.DiskUsage](t, `{"Volumes":[
				{"Name":"shop_data","UsageData":{"Size":1500000000}},
				{"Name":"shop_logs","UsageData":{"Size":500000000}},
				{"Name":"blog_data","UsageData":{"Size":7000000000}}
			]}`),
			Containers: decode[[]docker.Container](t, `[
				{"Id":"a1","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"},"Mounts":[{"Type":"volume","Name":"shop_data"},{"Type":"bind","Name":""}]},
				{"Id":"b2","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"},"Mounts":[{"Type":"volume","Name":"shop_data"},{"Type":"volume","Name":"shop_logs"}]}
			]`),
		}
		want := Service{Name: "api", State: "exited", DiskGB: 2}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add the bytes of its directory to the size of its named volumes in GB", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{
			DiskUsage: decode[docker.DiskUsage](t, `{"Volumes":[{"Name":"shop_data","UsageData":{"Size":1500000000}}]}`),
			Containers: decode[[]docker.Container](t, `[
				{"Id":"a1","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"},"Mounts":[{"Type":"volume","Name":"shop_data"},{"Type":"bind","Name":""}]}
			]`),
		}
		want := Service{Name: "api", State: "exited", DiskGB: 2.25}

		service := NewService("api", swarmService, snapshot, 750000000)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should count a negative volume size as 0", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{
			DiskUsage: decode[docker.DiskUsage](t, `{"Volumes":[
				{"Name":"shop_data","UsageData":{"Size":-1}},
				{"Name":"shop_logs","UsageData":{"Size":500000000}}
			]}`),
			Containers: decode[[]docker.Container](t, `[
				{"Id":"a1","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"},"Mounts":[{"Type":"volume","Name":"shop_data"},{"Type":"volume","Name":"shop_logs"}]}
			]`),
		}
		want := Service{Name: "api", State: "exited", DiskGB: 0.5}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should sum the stats of its containers that have stats", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{
			Containers: decode[[]docker.Container](t, `[
				{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}},
				{"Id":"b2","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}},
				{"Id":"c3","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"}}
			]`),
			Stats: map[string]docker.ContainerStats{
				"a1": decode[docker.ContainerStats](t, `{"memory_stats":{"usage":104857600}}`),
				"b2": decode[docker.ContainerStats](t, `{"memory_stats":{"usage":52428800}}`),
			},
		}
		want := Service{Name: "api", State: "running", MemoryUsedMiB: 150}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should not add the stats of the containers of other services", func(t *testing.T) {
		swarmService := decode[docker.SwarmService](t, `{"ID":"s1","Spec":{"Name":"shop_api"}}`)
		snapshot := docker.Snapshot{
			Containers: decode[[]docker.Container](t, `[
				{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}},
				{"Id":"z9","State":"running","Labels":{"com.docker.swarm.service.name":"shop_web"}}
			]`),
			Stats: map[string]docker.ContainerStats{
				"a1": decode[docker.ContainerStats](t, `{"memory_stats":{"usage":104857600}}`),
				"z9": decode[docker.ContainerStats](t, `{"memory_stats":{"usage":52428800}}`),
			},
		}
		want := Service{Name: "api", State: "running", MemoryUsedMiB: 100}

		service := NewService("api", swarmService, snapshot, 0)

		if service != want {
			t.Errorf("NewService() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add the memory used without the inactive file of cgroup v2", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{"memory_stats":{"usage":314572800,"stats":{"inactive_file":104857600}}}`)
		service := Service{}
		want := Service{MemoryUsedMiB: 200}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add the memory used without the total inactive file of cgroup v1 over the inactive file of cgroup v2", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{"memory_stats":{"usage":314572800,"stats":{"total_inactive_file":104857600,"inactive_file":52428800}}}`)
		service := Service{}
		want := Service{MemoryUsedMiB: 200}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should subtract the inactive file of cgroup v2 when the total inactive file of cgroup v1 is not below the usage", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{"memory_stats":{"usage":314572800,"stats":{"total_inactive_file":314572800,"inactive_file":104857600}}}`)
		service := Service{}
		want := Service{MemoryUsedMiB: 200}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add the whole usage when the inactive file of cgroup v2 is not below the usage", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{"memory_stats":{"usage":314572800,"stats":{"inactive_file":314572800}}}`)
		service := Service{}
		want := Service{MemoryUsedMiB: 300}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add the CPU percent where 100 is one core", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{
			"cpu_stats":{"cpu_usage":{"total_usage":1250000000},"system_cpu_usage":8000000000,"online_cpus":4},
			"precpu_stats":{"cpu_usage":{"total_usage":1000000000},"system_cpu_usage":7000000000}
		}`)
		service := Service{}
		want := Service{CPUPercent: 100}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should count the CPUs of the per-CPU usage when there are no online CPUs", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{
			"cpu_stats":{"cpu_usage":{"total_usage":1250000000,"percpu_usage":[700000000,550000000]},"system_cpu_usage":8000000000},
			"precpu_stats":{"cpu_usage":{"total_usage":1000000000},"system_cpu_usage":7000000000}
		}`)
		service := Service{}
		want := Service{CPUPercent: 50}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add no CPU when the CPU usage of the container did not grow", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{
			"cpu_stats":{"cpu_usage":{"total_usage":1000000000},"system_cpu_usage":8000000000,"online_cpus":4},
			"precpu_stats":{"cpu_usage":{"total_usage":1000000000},"system_cpu_usage":7000000000}
		}`)
		service := Service{}
		want := Service{}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add no CPU when the CPU usage of the system did not grow", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{
			"cpu_stats":{"cpu_usage":{"total_usage":1250000000},"system_cpu_usage":7000000000,"online_cpus":4},
			"precpu_stats":{"cpu_usage":{"total_usage":1000000000},"system_cpu_usage":7000000000}
		}`)
		service := Service{}
		want := Service{}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add the network received and sent in MB", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{"networks":{
			"eth0":{"rx_bytes":1500000,"tx_bytes":250000},
			"eth1":{"rx_bytes":500000,"tx_bytes":250000}
		}}`)
		service := Service{}
		want := Service{NetRxMB: 2, NetTxMB: 0.5}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})

	t.Run("Should add the block read and written in MB, matching the operation in any case", func(t *testing.T) {
		stats := decode[docker.ContainerStats](t, `{"blkio_stats":{"io_service_bytes_recursive":[
			{"op":"Read","value":3000000},
			{"op":"Write","value":1000000},
			{"op":"read","value":1000000},
			{"op":"write","value":500000},
			{"op":"Total","value":5500000}
		]}}`)
		service := Service{}
		want := Service{BlockReadMB: 4, BlockWriteMB: 1.5}

		service.addStats(stats)

		if service != want {
			t.Errorf("addStats() = %+v, want %+v", service, want)
		}
	})
}
