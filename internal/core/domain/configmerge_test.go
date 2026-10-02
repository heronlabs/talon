package domain_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

type merge struct {
	Merged string
	Added  []string
}

func mergeOf(existing string, values map[string]string) merge {
	merged, added := domain.MergeConfig([]byte(existing), values)
	return merge{string(merged), added}
}

func TestMergeConfig_CreatesFromNothing(t *testing.T) {
	got := mergeOf("", map[string]string{"GITHUB_TOKEN": "t"})

	testkit.Equal(t, merge{"GITHUB_TOKEN=t\n", []string{"GITHUB_TOKEN"}}, got)
}

func TestMergeConfig_AppendsMissingKeyAtTheEnd(t *testing.T) {
	got := mergeOf("CMS_URL=https://cms\nAGENT_TOKEN=a\n", map[string]string{"GITHUB_TOKEN": "t"})

	testkit.Equal(t, merge{"CMS_URL=https://cms\nAGENT_TOKEN=a\nGITHUB_TOKEN=t\n", []string{"GITHUB_TOKEN"}}, got)
}

func TestMergeConfig_EndsTheLastLineBeforeAppending(t *testing.T) {
	got := mergeOf("CMS_URL=https://cms", map[string]string{"GITHUB_TOKEN": "t"})

	testkit.Equal(t, merge{"CMS_URL=https://cms\nGITHUB_TOKEN=t\n", []string{"GITHUB_TOKEN"}}, got)
}

func TestMergeConfig_KeepsAPresentKey(t *testing.T) {
	got := mergeOf("GITHUB_TOKEN=old\n", map[string]string{"GITHUB_TOKEN": "new"})

	testkit.Equal(t, merge{Merged: "GITHUB_TOKEN=old\n"}, got)
}

func TestMergeConfig_KeepsAPresentKeyWrittenWithSpaces(t *testing.T) {
	got := mergeOf("  GITHUB_TOKEN = old\n", map[string]string{"GITHUB_TOKEN": "new"})

	testkit.Equal(t, merge{Merged: "  GITHUB_TOKEN = old\n"}, got)
}

func TestMergeConfig_LeavesAFileWithoutNewlineAloneWhenNothingIsAdded(t *testing.T) {
	got := mergeOf("CMS_URL=https://cms", map[string]string{"CMS_URL": "other"})

	testkit.Equal(t, merge{Merged: "CMS_URL=https://cms"}, got)
}

func TestMergeConfig_KeepsCommentsAndAddsAKeyOnlyCommentedOut(t *testing.T) {
	got := mergeOf("# written by install.sh\n#GITHUB_TOKEN=x\n", map[string]string{"GITHUB_TOKEN": "t"})

	testkit.Equal(t, merge{"# written by install.sh\n#GITHUB_TOKEN=x\nGITHUB_TOKEN=t\n", []string{"GITHUB_TOKEN"}}, got)
}

func TestMergeConfig_IgnoresEmptyValues(t *testing.T) {
	got := mergeOf("CMS_URL=https://cms\n", map[string]string{"AGENT_TOKEN": "", "GITHUB_TOKEN": "t"})

	testkit.Equal(t, merge{"CMS_URL=https://cms\nGITHUB_TOKEN=t\n", []string{"GITHUB_TOKEN"}}, got)
}

func TestMergeConfig_AppendsInKeyOrder(t *testing.T) {
	got := mergeOf("", map[string]string{"GITHUB_TOKEN": "t", "AGENT_HOST_NAME": "srv"})

	testkit.Equal(t, merge{"AGENT_HOST_NAME=srv\nGITHUB_TOKEN=t\n", []string{"AGENT_HOST_NAME", "GITHUB_TOKEN"}}, got)
}
