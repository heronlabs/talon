package domain

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// MergeConfig appends to existing a KEY=VALUE line for every key of values that has a value and
// is not set in existing yet, in key order. Lines already there are never rewritten or removed,
// comments and order included. added lists the appended keys, never their values.
func MergeConfig(existing []byte, values map[string]string) (merged []byte, added []string) {
	present := configKeys(existing)
	var tail bytes.Buffer
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if values[key] == "" || present[key] {
			continue
		}
		added = append(added, key)
		fmt.Fprintf(&tail, "%s=%s\n", key, values[key])
	}
	if len(added) == 0 {
		return existing, nil
	}
	merged = bytes.Clone(existing)
	if len(merged) > 0 && !bytes.HasSuffix(merged, []byte("\n")) {
		merged = append(merged, '\n')
	}
	return append(merged, tail.Bytes()...), added
}

// configKeys lists the text before "=" on every line. A comment line keeps its "#", so it never
// matches a key.
func configKeys(existing []byte) map[string]bool {
	keys := map[string]bool{}
	for line := range strings.Lines(string(existing)) {
		key, _, _ := strings.Cut(line, "=")
		keys[strings.TrimSpace(key)] = true
	}
	return keys
}
