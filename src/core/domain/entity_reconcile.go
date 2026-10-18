package domain

import (
	"slices"

	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
)

// The states of the reconcile of a project: not in the manifest, at least one declared service and every one matching its configuration, or not.
const (
	reconcileUnmanaged = "unmanaged"
	reconcileConverged = "converged"
	reconcilePending   = "pending"
)

// ignoredKeys are the keys of the declared data that are never compared, at any depth: projectName and serviceName, which identify the service, and env and password, which the CMS refuses.
var ignoredKeys = []string{"projectName", "serviceName", "env", "password"}

// Reconcile holds the state of the reconcile of a project, the secrets it misses and its error.
type Reconcile struct {
	State          string   `json:"state"`
	MissingSecrets []string `json:"missingSecrets"`
	Error          *string  `json:"error"`
}

// NewReconcile returns the reconcile of the project name: nil without configured, as without MANIFEST_URL or when the observation failed; otherwise unmanaged without client, its entry in the manifest, converged when client declares at least one service and every one matches the service of the same name configured holds in the project, and pending when it declares none or one does not match, with no missing secret and no error.
func NewReconcile(name string, client *manifest.Client, configured *easypanel.ConfigSnapshot) *Reconcile {
	if configured == nil {
		return nil
	}

	return &Reconcile{State: reconcileState(client, configured.Services[name]), MissingSecrets: []string{}}
}

// reconcileState returns unmanaged without client, converged when it declares at least one service and every one matches its service in configured, and pending otherwise.
func reconcileState(client *manifest.Client, configured map[string]easypanel.ServiceConfig) string {
	if client == nil {
		return reconcileUnmanaged
	}

	if converged(client.Schema.Services, configured) {
		return reconcileConverged
	}

	return reconcilePending
}

// converged reports whether declared holds at least one service and every one matches its service in configured.
func converged(declared []manifest.Service, configured map[string]easypanel.ServiceConfig) bool {
	if len(declared) == 0 {
		return false
	}

	for _, service := range declared {
		if !serviceMatches(service, configured) {
			return false
		}
	}

	return true
}

// serviceMatches reports whether configured holds the service named as declared, with its type and data matching the declared ones.
func serviceMatches(declared manifest.Service, configured map[string]easypanel.ServiceConfig) bool {
	service, found := configured[declaredName(declared)]

	return found && service.Type == declared.Type && matches(declared.Data, service.Data)
}

// matches reports whether configured matches declared: an object on its declared keys but ignoredKeys, recursively, an array item by item in any order, a null as null or an absent key, and any other value as an equal value.
func matches(declared, configured any) bool {
	switch declared := declared.(type) {
	case map[string]any:
		return objectMatches(declared, configured)
	case []any:
		return arrayMatches(declared, configured)
	default:
		return declared == configured
	}
}

// objectMatches reports whether configured is an object whose value of each key of declared but ignoredKeys matches the declared one, an absent key reading as null.
func objectMatches(declared map[string]any, configured any) bool {
	object, isObject := configured.(map[string]any)
	if !isObject {
		return false
	}

	for key, value := range declared {
		if !slices.Contains(ignoredKeys, key) && !matches(value, object[key]) {
			return false
		}
	}

	return true
}

// arrayMatches reports whether configured is an array of the length of declared in which each declared item matches a distinct item.
func arrayMatches(declared []any, configured any) bool {
	items, isArray := configured.([]any)

	return isArray && len(items) == len(declared) && assign(declared, items, make([]bool, len(items)))
}

// assign reports whether each item of declared matches a distinct item of configured that is not used yet, trying every item for each.
func assign(declared, configured []any, used []bool) bool {
	if len(declared) == 0 {
		return true
	}

	for index, item := range configured {
		if !used[index] && matches(declared[0], item) {
			used[index] = true
			if assign(declared[1:], configured, used) {
				return true
			}
			used[index] = false
		}
	}

	return false
}
