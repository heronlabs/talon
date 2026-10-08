package docker

// Container is an item of the answer of the Engine API that lists the containers: its ID, state, labels and mounts.
type Container struct {
	ID     string `json:"Id"`
	State  string
	Labels map[string]string
	Mounts []struct {
		Type string
		Name string
	}
}
