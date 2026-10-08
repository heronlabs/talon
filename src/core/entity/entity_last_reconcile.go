package entity

// LastReconcile holds when the last reconcile ran, its outcome and its error, if any.
type LastReconcile struct {
	At      string  `json:"at"`
	Outcome string  `json:"outcome"`
	Error   *string `json:"error,omitempty"`
}
