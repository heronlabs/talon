package easypanel

// ConfigSnapshot holds the configuration Easypanel holds for its app, postgres and mysql services.
type ConfigSnapshot struct {
	// Services are the configuration of each service, by project and then by service.
	Services map[string]map[string]ServiceConfig
}
