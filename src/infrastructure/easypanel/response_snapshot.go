package easypanel

// Snapshot holds the directories of the Easypanel projects under the root of the host.
type Snapshot struct {
	// Sizes are the bytes of the regular files under the directory of each service, by project and then by service.
	Sizes map[string]map[string]int64
}
