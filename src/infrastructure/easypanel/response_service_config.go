package easypanel

// ServiceConfig is the configuration of a service that Easypanel holds, in the shape of a service of its schema: its type and its data, decoded as JSON values.
type ServiceConfig struct {
	Type string
	Data map[string]any
}
