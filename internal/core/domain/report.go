package domain

import "time"

// ServiceState is the state reported for a service, following its containers.
type ServiceState string

// Service states, in the contract's vocabulary.
const (
	StateRunning    ServiceState = "running"
	StateRestarting ServiceState = "restarting"
	StatePaused     ServiceState = "paused"
	StateCreated    ServiceState = "created"
	StateExited     ServiceState = "exited"
	StateDead       ServiceState = "dead"
	StateMissing    ServiceState = "missing"
)

// ReconcileState is the reconcile outcome of a project.
type ReconcileState string

// StateUnmanaged marks a project measured but not reconciled by the agent.
const StateUnmanaged ReconcileState = "unmanaged"

// Report is one measurement of the host and of every Easypanel project on it.
type Report struct {
	Host         string
	MeasuredAt   time.Time
	AgentVersion string
	System       System
	Projects     []Project
}

// System is the host-wide part of the report.
type System struct {
	RAMTotal         Bytes
	RAMUsed          Bytes
	SwapTotal        Bytes
	SwapUsed         Bytes
	DiskTotal        Bytes
	DiskUsed         Bytes
	Load1            float64
	Load5            float64
	Load15           float64
	Uptime           time.Duration
	DockerImages     Bytes
	DockerBuildCache Bytes
}

// Project is an Easypanel project and its services.
type Project struct {
	Name      string
	Reconcile ReconcileState
	Services  []Service
}

// Service is the measurement of one Swarm service.
type Service struct {
	Name               string
	Managed            bool
	State              ServiceState
	ReplicasRunning    int
	ReplicasDesired    int
	RecentTaskFailures int
	MemoryUsed         Bytes
	MemoryLimit        Bytes
	CPUPercent         float64
	CPULimit           float64
	Disk               Bytes
	NetRx              Bytes
	NetTx              Bytes
	BlockRead          Bytes
	BlockWrite         Bytes
}
