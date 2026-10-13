package easypanel

import (
	"context"
	"maps"
	"net/url"
	"slices"
)

// The operations of the Easypanel API that list the services and the domains of a service, the query parameters that name a service, the type of an app and the key of its domains in its data.
const (
	listOperation        = "listProjectsAndServices"
	listDomainsOperation = "listDomains"
	projectNameParameter = "projectName"
	serviceNameParameter = "serviceName"
	appType              = "app"
	domainsKey           = "domains"
)

// inspectOperations are the operations of the Easypanel API that inspect a service, by the types of service the collector reads.
var inspectOperations = map[string]string{
	appType:    "inspectAppService",
	"postgres": "inspectPostgresService",
	"mysql":    "inspectMySQLService",
}

// droppedKeys are the keys of an inspected service that its data leaves out: env and password, which the schema never declares, and the deploy token and URL, which are not part of it.
var droppedKeys = []string{"env", "password", "token", "deploymentUrl"}

// ConfigCollectorService collects through the Easypanel API the configuration Easypanel holds for its services, in the shape of its schema.
type ConfigCollectorService struct {
	client *APIClient
}

// NewConfigCollectorService returns a ConfigCollectorService that requests the Easypanel API through client.
func NewConfigCollectorService(client *APIClient) *ConfigCollectorService {
	return &ConfigCollectorService{client: client}
}

// Collect lists the services of the Easypanel API at endpoint with token and ctx, reads each app, postgres or mysql service with the inspect operation of its type and the domains of each app, and returns their configuration in the shape of the schema; the first request that fails fails it.
func (s *ConfigCollectorService) Collect(ctx context.Context, endpoint, token string) (*ConfigSnapshot, error) {
	var list ProjectsAndServices
	if err := s.client.Get(ctx, endpoint, token, listOperation, nil, &list); err != nil {
		return nil, err
	}

	services := map[string]map[string]ServiceConfig{}
	for _, service := range list.Services {
		if err := s.collectService(ctx, endpoint, token, service, services); err != nil {
			return nil, err
		}
	}

	return &ConfigSnapshot{Services: services}, nil
}

// collectService adds to services, under its project, the configuration of service when the collector reads its type.
func (s *ConfigCollectorService) collectService(ctx context.Context, endpoint, token string, service ListedService, services map[string]map[string]ServiceConfig) error {
	operation, read := inspectOperations[service.Type]
	if !read {
		return nil
	}

	data, err := s.inspect(ctx, endpoint, token, operation, service)
	if err != nil {
		return err
	}

	if services[service.ProjectName] == nil {
		services[service.ProjectName] = map[string]ServiceConfig{}
	}
	services[service.ProjectName][service.Name] = ServiceConfig{Type: service.Type, Data: data}

	return nil
}

// inspect returns the data of service, its answer to operation without droppedKeys and, for an app, with its domains.
func (s *ConfigCollectorService) inspect(ctx context.Context, endpoint, token, operation string, service ListedService) (map[string]any, error) {
	query := url.Values{projectNameParameter: {service.ProjectName}, serviceNameParameter: {service.Name}}
	var answer map[string]any
	if err := s.client.Get(ctx, endpoint, token, operation, query, &answer); err != nil {
		return nil, err
	}

	data := serviceData(answer)
	if service.Type != appType {
		return data, nil
	}

	domains, err := s.domains(ctx, endpoint, token, query)
	if err != nil {
		return nil, err
	}
	data[domainsKey] = domains

	return data, nil
}

// domains returns the domains of the service named by query, from listDomains, in the shape of the domains of the schema.
func (s *ConfigCollectorService) domains(ctx context.Context, endpoint, token string, query url.Values) ([]any, error) {
	var answer []Domain
	if err := s.client.Get(ctx, endpoint, token, listDomainsOperation, query, &answer); err != nil {
		return nil, err
	}

	domains := make([]any, 0, len(answer))
	for _, domain := range answer {
		domains = append(domains, schemaDomain(domain))
	}

	return domains, nil
}

// serviceData returns a copy of answer, the inspect answer of a service, without droppedKeys.
func serviceData(answer map[string]any) map[string]any {
	data := map[string]any{}
	maps.Copy(data, answer)
	maps.DeleteFunc(data, func(key string, _ any) bool { return slices.Contains(droppedKeys, key) })

	return data
}

// schemaDomain returns domain in the shape of a domain of the schema, whose port and internal protocol are those of the service it routes to.
func schemaDomain(domain Domain) map[string]any {
	return map[string]any{
		"host":                domain.Host,
		"https":               domain.HTTPS,
		"port":                domain.ServiceDestination.Port,
		"path":                domain.Path,
		"internalProtocol":    domain.ServiceDestination.Protocol,
		"certificateResolver": domain.CertificateResolver,
		"middlewares":         domain.Middlewares,
		"wildcard":            domain.Wildcard,
	}
}
