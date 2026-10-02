package domain

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ErrInvalidVersion marks a version that is not exactly MAJOR.MINOR.PATCH.
var ErrInvalidVersion = errors.New("invalid version")

// ErrBelowMinimum marks a target older than MinUpdatableVersion. Its text completes
// "version X ", so the error reads "version 0.1.0 is older than 0.2.0, ...".
var ErrBelowMinimum = errors.New("is older than 0.2.0, the first one update can install")

// MinUpdatableVersion is the oldest release update can install: older ones do not know GITHUB_TOKEN.
var MinUpdatableVersion = Version{Major: 0, Minor: 2, Patch: 0}

var versionPattern = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)$`)

// Version is a release version, MAJOR.MINOR.PATCH.
type Version struct {
	Major, Minor, Patch int
}

// ParseVersion reads exactly MAJOR.MINOR.PATCH: no "v", no spaces, nothing else.
func ParseVersion(s string) (Version, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("%w: %q", ErrInvalidVersion, s)
	}
	major, errMajor := strconv.Atoi(m[1])
	minor, errMinor := strconv.Atoi(m[2])
	patch, errPatch := strconv.Atoi(m[3])
	if err := errors.Join(errMajor, errMinor, errPatch); err != nil {
		return Version{}, fmt.Errorf("%w: %q: %w", ErrInvalidVersion, s, err)
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// ParseTag reads a release tag, vMAJOR.MINOR.PATCH.
func ParseTag(tag string) (Version, error) {
	rest, ok := strings.CutPrefix(tag, "v")
	if !ok {
		return Version{}, fmt.Errorf("%w: tag %q has no leading v", ErrInvalidVersion, tag)
	}
	return ParseVersion(rest)
}

// String renders MAJOR.MINOR.PATCH.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1, 0 or +1 as v is older than, equal to or newer than o.
func (v Version) Compare(o Version) int {
	return cmp.Or(cmp.Compare(v.Major, o.Major), cmp.Compare(v.Minor, o.Minor), cmp.Compare(v.Patch, o.Patch))
}

// Decision is what update does with a target version.
type Decision string

// The decisions UpdatePlan takes.
const (
	DecisionInstall        Decision = "install"
	DecisionAlreadyCurrent Decision = "already-current"
)

// UpdatePlan decides whether update installs target over current. Downgrades are allowed: they
// are the manual rollback.
func UpdatePlan(current, target Version) (Decision, error) {
	if target.Compare(MinUpdatableVersion) < 0 {
		return "", fmt.Errorf("version %s %w", target, ErrBelowMinimum)
	}
	if target == current {
		return DecisionAlreadyCurrent, nil
	}
	return DecisionInstall, nil
}
