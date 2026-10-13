package entity

import "github.com/heronlabs/talon/src/infrastructure/manifest"

// The outcomes of a reconcile and the longest error, in characters, that the CMS accepts.
const (
	outcomeOK      = "ok"
	outcomeFailed  = "failed"
	maxErrorLength = 500
)

// LastReconcile holds when the last reconcile ran, its outcome and its error, if any.
type LastReconcile struct {
	At      string  `json:"at"`
	Outcome string  `json:"outcome"`
	Error   *string `json:"error"`
}

// NewLastReconcile returns the reconcile that ran at at: failed with err, its message cut to maxErrorLength characters, when the observation failed; nil without declared, as without MANIFEST_URL; and ok otherwise.
func NewLastReconcile(at string, declared *manifest.Manifest, err error) *LastReconcile {
	if err != nil {
		message := truncate(err.Error(), maxErrorLength)
		return &LastReconcile{At: at, Outcome: outcomeFailed, Error: &message}
	}

	if declared == nil {
		return nil
	}

	return &LastReconcile{At: at, Outcome: outcomeOK}
}

// truncate returns the first limit characters of text, or text when it is shorter.
func truncate(text string, limit int) string {
	characters := []rune(text)

	return string(characters[:min(len(characters), limit)])
}
