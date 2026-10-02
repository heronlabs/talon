package config_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/heronlabs/vps-agent/internal/infrastructure/config"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const full = `# written by install.sh
CMS_URL=https://cms.heronlabs.com.br

AGENT_TOKEN=token
AGENT_HOST_NAME=vps-heron-02
EASYPANEL_URL=http://127.0.0.1:3001
EASYPANEL_TOKEN=ep
AWS_ACCESS_KEY_ID=AKIA
AWS_SECRET_ACCESS_KEY=secret
AWS_REGION=sa-east-1
RECONCILE_INTERVAL=1m
REPORT_INTERVAL=2m
`

var errFallback = errors.New("fallback")

func dir(content string, mode fs.FileMode) fstest.MapFS {
	return fstest.MapFS{"config.env": {Data: []byte(content), Mode: mode}}
}

func parse(content string) (config.Config, error) {
	return config.Parse(strings.NewReader(content))
}

func hostname() (string, error) { return "fallback-host", nil }

// unreadable stats like a normal file but cannot be opened.
type unreadable struct{ files fstest.MapFS }

func (u unreadable) Open(string) (fs.File, error) { return nil, fs.ErrPermission }

func (u unreadable) Stat(name string) (fs.FileInfo, error) { return u.files.Stat(name) }

func TestParse_ReadsEveryKey(t *testing.T) {
	got := testkit.Must(parse(full))

	testkit.Equal(t, config.Config{
		CMSURL: "https://cms.heronlabs.com.br", AgentToken: "token", HostName: "vps-heron-02",
		EasypanelURL: "http://127.0.0.1:3001", EasypanelToken: "ep",
		AWSAccessKeyID: "AKIA", AWSSecretAccessKey: "secret", AWSRegion: "sa-east-1",
		ReconcileInterval: time.Minute, ReportInterval: 2 * time.Minute,
	}, got)
}

func TestParse_EmptyFileGivesDefaults(t *testing.T) {
	got := testkit.Must(parse(""))

	testkit.Equal(t, config.Config{
		EasypanelURL: "http://127.0.0.1:3000", AWSRegion: "us-east-1",
		ReconcileInterval: 5 * time.Minute, ReportInterval: 10 * time.Minute,
	}, got)
}

func TestParse_KeepsEqualsSignsInsideTheValue(t *testing.T) {
	got := testkit.Must(parse("AGENT_TOKEN=a=b\n"))

	testkit.Equal(t, "a=b", got.AgentToken)
}

func TestParse_AcceptsWindowsLineEndings(t *testing.T) {
	got := testkit.Must(parse("CMS_URL=https://cms\r\nREPORT_INTERVAL=1m\r\n"))

	testkit.Equal(t, "https://cms", got.CMSURL)
}

func TestParse_DropsSpacesAroundKeyAndValue(t *testing.T) {
	got := testkit.Must(parse("CMS_URL = https://cms \n"))

	testkit.Equal(t, "https://cms", got.CMSURL)
}

func TestParse_DropsOnePairOfDoubleQuotes(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN="a b"` + "\n"))

	testkit.Equal(t, "a b", got.AgentToken)
}

func TestParse_EmptyQuotedValueIsEmpty(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN=""` + "\n"))

	testkit.Equal(t, "", got.AgentToken)
}

func TestParse_KeepsALoneDoubleQuote(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN="` + "\n"))

	testkit.Equal(t, `"`, got.AgentToken)
}

func TestParse_KeepsQuotesThatDoNotWrapTheValue(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN="a"b` + "\n"))

	testkit.Equal(t, `"a"b`, got.AgentToken)
}

func TestParse_RejectsLineWithoutEquals(t *testing.T) {
	_, err := parse("CMS_URL\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_NamesTheLineWithoutEquals(t *testing.T) {
	_, err := parse("# comment\nCMS_URL\n")

	testkit.Equal(t, "invalid config: line 2 has no '='", err.Error())
}

func TestParse_RejectsUnknownKey(t *testing.T) {
	_, err := parse("CMS_ULR=https://cms\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsRepeatedKey(t *testing.T) {
	_, err := parse("CMS_URL=a\nCMS_URL=b\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsInvalidDuration(t *testing.T) {
	_, err := parse("REPORT_INTERVAL=ten\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsZeroDuration(t *testing.T) {
	_, err := parse("RECONCILE_INTERVAL=0s\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsOverlongLine(t *testing.T) {
	_, err := parse("AGENT_TOKEN=" + strings.Repeat("x", 70*1024) + "\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestLoad_ReadsAPrivateFile(t *testing.T) {
	got := testkit.Must(config.Load(dir("AGENT_HOST_NAME=vps-heron-02\n", 0o600), "config.env"))

	testkit.Equal(t, "vps-heron-02", got.HostName)
}

func TestLoad_RejectsFileOthersCanRead(t *testing.T) {
	_, err := config.Load(dir("", 0o644), "config.env")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestLoad_FailsWhenFileIsMissing(t *testing.T) {
	_, err := config.Load(fstest.MapFS{}, "config.env")

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestLoad_FailsWhenFileCannotBeRead(t *testing.T) {
	_, err := config.Load(unreadable{dir("", 0o600)}, "config.env")

	testkit.ErrorIs(t, err, fs.ErrPermission)
}

func TestFileValidate_PassesOnValidFile(t *testing.T) {
	err := config.File{Dir: dir(full, 0o600), Name: "config.env"}.Validate()

	testkit.ErrorIs(t, err, nil)
}

func TestFileValidate_FailsOnInvalidFile(t *testing.T) {
	err := config.File{Dir: dir("NOPE=1\n", 0o600), Name: "config.env"}.Validate()

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestHostNameOr_PrefersTheConfiguredName(t *testing.T) {
	got := testkit.Must(config.File{Dir: dir(full, 0o600), Name: "config.env"}.HostNameOr(hostname))

	testkit.Equal(t, "vps-heron-02", got)
}

func TestHostNameOr_FallsBackWhenNameIsUnset(t *testing.T) {
	got := testkit.Must(config.File{Dir: dir("", 0o600), Name: "config.env"}.HostNameOr(hostname))

	testkit.Equal(t, "fallback-host", got)
}

func TestHostNameOr_FallsBackWhenFileIsMissing(t *testing.T) {
	_, err := config.File{Dir: fstest.MapFS{}, Name: "config.env"}.HostNameOr(func() (string, error) {
		return "", errFallback
	})

	testkit.ErrorIs(t, err, errFallback)
}
