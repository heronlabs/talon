package entity

// Reconcile holds the state of the reconcile of a project, the secrets it misses and its error.
type Reconcile struct {
	State          string   `json:"state"`
	MissingSecrets []string `json:"missingSecrets"`
	Error          *string  `json:"error"`
}
