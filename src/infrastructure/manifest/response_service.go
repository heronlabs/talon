package manifest

// Service is a service of the schema of a client: its type, app, postgres or mysql, and its data, the declared configuration of the service with its projectName and serviceName, decoded as JSON values.
type Service struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}
