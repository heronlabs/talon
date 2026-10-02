package domain_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

type plan struct {
	Decision domain.Decision
	Err      string
}

func planOf(current, target domain.Version) plan {
	d, err := domain.UpdatePlan(current, target)
	if err != nil {
		return plan{Decision: d, Err: err.Error()}
	}
	return plan{Decision: d}
}

func TestParseVersion_ReadsMajorMinorPatch(t *testing.T) {
	got := testkit.Must(domain.ParseVersion("1.22.333"))

	testkit.Equal(t, domain.Version{Major: 1, Minor: 22, Patch: 333}, got)
}

func TestParseVersion_RejectsLeadingV(t *testing.T) {
	_, err := domain.ParseVersion("v1.2.3")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseVersion_RejectsSpace(t *testing.T) {
	_, err := domain.ParseVersion("1.2.3 ")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseVersion_RejectsTrailingCommand(t *testing.T) {
	_, err := domain.ParseVersion("1.2.3;id")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseVersion_RejectsTrailingNewline(t *testing.T) {
	_, err := domain.ParseVersion("1.2.3\n")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseVersion_RejectsEmpty(t *testing.T) {
	_, err := domain.ParseVersion("")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseVersion_RejectsMissingPatch(t *testing.T) {
	_, err := domain.ParseVersion("1.2")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseVersion_RejectsPrerelease(t *testing.T) {
	_, err := domain.ParseVersion("0.0.0-dev")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseVersion_RejectsNumberThatOverflows(t *testing.T) {
	_, err := domain.ParseVersion("99999999999999999999.0.0")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseTag_ReadsTheVersionAfterV(t *testing.T) {
	got := testkit.Must(domain.ParseTag("v0.2.1"))

	testkit.Equal(t, domain.Version{Major: 0, Minor: 2, Patch: 1}, got)
}

func TestParseTag_RejectsTagWithoutV(t *testing.T) {
	_, err := domain.ParseTag("0.2.1")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestParseTag_RejectsInvalidVersion(t *testing.T) {
	_, err := domain.ParseTag("v0.2")

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestVersionString_JoinsWithDots(t *testing.T) {
	got := domain.Version{Major: 1, Minor: 20, Patch: 3}.String()

	testkit.Equal(t, "1.20.3", got)
}

func TestVersionCompare_MajorDecidesFirst(t *testing.T) {
	got := domain.Version{Major: 2}.Compare(domain.Version{Major: 1, Minor: 9, Patch: 9})

	testkit.Equal(t, 1, got)
}

func TestVersionCompare_MinorDecidesWhenMajorsMatch(t *testing.T) {
	got := domain.Version{Major: 1, Minor: 1}.Compare(domain.Version{Major: 1, Minor: 2})

	testkit.Equal(t, -1, got)
}

func TestVersionCompare_PatchDecidesWhenMinorsMatch(t *testing.T) {
	got := domain.Version{Major: 1, Minor: 2, Patch: 4}.Compare(domain.Version{Major: 1, Minor: 2, Patch: 3})

	testkit.Equal(t, 1, got)
}

func TestVersionCompare_EqualVersionsAreZero(t *testing.T) {
	got := domain.Version{Major: 1, Minor: 2, Patch: 3}.Compare(domain.Version{Major: 1, Minor: 2, Patch: 3})

	testkit.Equal(t, 0, got)
}

func TestUpdatePlan_RefusesVersionBelowTheMinimum(t *testing.T) {
	_, err := domain.UpdatePlan(domain.Version{Minor: 2}, domain.Version{Minor: 1, Patch: 9})

	testkit.ErrorIs(t, err, domain.ErrBelowMinimum)
}

func TestUpdatePlan_NamesTheVersionBelowTheMinimum(t *testing.T) {
	got := planOf(domain.Version{Minor: 2}, domain.Version{Minor: 1})

	testkit.Equal(t, plan{Err: "version 0.1.0 is older than 0.2.0, the first one update can install"}, got)
}

func TestUpdatePlan_InstallsTheMinimumItself(t *testing.T) {
	got := planOf(domain.Version{Minor: 1}, domain.Version{Minor: 2})

	testkit.Equal(t, plan{Decision: domain.DecisionInstall}, got)
}

func TestUpdatePlan_InstallsANewerVersion(t *testing.T) {
	got := planOf(domain.Version{Minor: 2}, domain.Version{Minor: 3})

	testkit.Equal(t, plan{Decision: domain.DecisionInstall}, got)
}

func TestUpdatePlan_InstallsAnOlderVersionAsRollback(t *testing.T) {
	got := planOf(domain.Version{Minor: 3}, domain.Version{Minor: 2})

	testkit.Equal(t, plan{Decision: domain.DecisionInstall}, got)
}

func TestUpdatePlan_SkipsTheRunningVersion(t *testing.T) {
	got := planOf(domain.Version{Minor: 3}, domain.Version{Minor: 3})

	testkit.Equal(t, plan{Decision: domain.DecisionAlreadyCurrent}, got)
}
