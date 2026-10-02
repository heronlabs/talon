// Package cms holds the CMS contract DTOs and the adapters that publish reports.
package cms

import (
	"math"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

const (
	gib = 1 << 30
	mib = 1 << 20
	gb  = 1e9
	mb  = 1e6
)

// ReportDTO is the body of POST /api/infra-reports.
type ReportDTO struct {
	Host       string       `json:"host"`
	MeasuredAt string       `json:"measuredAt"`
	Agent      AgentDTO     `json:"agent"`
	System     SystemDTO    `json:"system"`
	Projects   []ProjectDTO `json:"projects"`
}

// AgentDTO describes the agent. Easypanel fields stay null until the agent talks to Easypanel.
type AgentDTO struct {
	Version            string    `json:"version"`
	EasypanelVersion   *string   `json:"easypanelVersion"`
	EasypanelSupported *bool     `json:"easypanelSupported"`
	LastReconcile      *struct{} `json:"lastReconcile"`
}

// SystemDTO is the host block of the report.
type SystemDTO struct {
	RAMTotalGiB        float64 `json:"ramTotalGiB"`
	RAMUsedGiB         float64 `json:"ramUsedGiB"`
	SwapTotalGiB       float64 `json:"swapTotalGiB"`
	SwapUsedGiB        float64 `json:"swapUsedGiB"`
	DiskTotalGB        float64 `json:"diskTotalGB"`
	DiskUsedGB         float64 `json:"diskUsedGB"`
	Load1              float64 `json:"load1"`
	Load5              float64 `json:"load5"`
	Load15             float64 `json:"load15"`
	UptimeSeconds      int64   `json:"uptimeSeconds"`
	DockerImagesGB     float64 `json:"dockerImagesGB"`
	DockerBuildCacheGB float64 `json:"dockerBuildCacheGB"`
}

// ProjectDTO is one Easypanel project of the report.
type ProjectDTO struct {
	Name      string       `json:"name"`
	Reconcile ReconcileDTO `json:"reconcile"`
	Services  []ServiceDTO `json:"services"`
}

// ReconcileDTO is the reconcile outcome of a project.
type ReconcileDTO struct {
	State          string   `json:"state"`
	MissingSecrets []string `json:"missingSecrets"`
	Error          *string  `json:"error"`
}

// ServiceDTO is one service of a project.
type ServiceDTO struct {
	Name               string  `json:"name"`
	Managed            bool    `json:"managed"`
	State              string  `json:"state"`
	ReplicasRunning    int     `json:"replicasRunning"`
	ReplicasDesired    int     `json:"replicasDesired"`
	RecentTaskFailures int     `json:"recentTaskFailures"`
	MemoryUsedMiB      float64 `json:"memoryUsedMiB"`
	MemoryLimitMiB     float64 `json:"memoryLimitMiB"`
	CPUPercent         float64 `json:"cpuPercent"`
	CPULimit           float64 `json:"cpuLimit"`
	DiskGB             float64 `json:"diskGB"`
	NetRxMB            float64 `json:"netRxMB"`
	NetTxMB            float64 `json:"netTxMB"`
	BlockReadMB        float64 `json:"blockReadMB"`
	BlockWriteMB       float64 `json:"blockWriteMB"`
}

// ToDTO converts a domain report into the contract, in its units, rounded to two decimals.
func ToDTO(r domain.Report) ReportDTO {
	projects := make([]ProjectDTO, 0, len(r.Projects))
	for _, p := range r.Projects {
		projects = append(projects, projectDTO(p))
	}
	return ReportDTO{
		Host:       r.Host,
		MeasuredAt: r.MeasuredAt.UTC().Format(time.RFC3339Nano),
		Agent:      AgentDTO{Version: r.AgentVersion},
		System: SystemDTO{
			RAMTotalGiB:        in(r.System.RAMTotal, gib),
			RAMUsedGiB:         in(r.System.RAMUsed, gib),
			SwapTotalGiB:       in(r.System.SwapTotal, gib),
			SwapUsedGiB:        in(r.System.SwapUsed, gib),
			DiskTotalGB:        in(r.System.DiskTotal, gb),
			DiskUsedGB:         in(r.System.DiskUsed, gb),
			Load1:              round2(r.System.Load1),
			Load5:              round2(r.System.Load5),
			Load15:             round2(r.System.Load15),
			UptimeSeconds:      int64(r.System.Uptime / time.Second),
			DockerImagesGB:     in(r.System.DockerImages, gb),
			DockerBuildCacheGB: in(r.System.DockerBuildCache, gb),
		},
		Projects: projects,
	}
}

func projectDTO(p domain.Project) ProjectDTO {
	services := make([]ServiceDTO, 0, len(p.Services))
	for _, s := range p.Services {
		services = append(services, ServiceDTO{
			Name:               s.Name,
			Managed:            s.Managed,
			State:              string(s.State),
			ReplicasRunning:    s.ReplicasRunning,
			ReplicasDesired:    s.ReplicasDesired,
			RecentTaskFailures: s.RecentTaskFailures,
			MemoryUsedMiB:      in(s.MemoryUsed, mib),
			MemoryLimitMiB:     in(s.MemoryLimit, mib),
			CPUPercent:         round2(s.CPUPercent),
			CPULimit:           round2(s.CPULimit),
			DiskGB:             in(s.Disk, gb),
			NetRxMB:            in(s.NetRx, mb),
			NetTxMB:            in(s.NetTx, mb),
			BlockReadMB:        in(s.BlockRead, mb),
			BlockWriteMB:       in(s.BlockWrite, mb),
		})
	}
	return ProjectDTO{
		Name:      p.Name,
		Reconcile: ReconcileDTO{State: string(p.Reconcile), MissingSecrets: []string{}},
		Services:  services,
	}
}

func in(b domain.Bytes, unit float64) float64 { return round2(float64(b) / unit) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }
