package process_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/infrastructure/process"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

// script writes an executable shell script and returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vps-agent")
	testkit.MustDo(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o600))
	testkit.MustDo(t, os.Chmod(path, 0o700)) //nolint:gosec // the script must be executable
	return path
}

func doctor(binary string) process.Doctor {
	return process.Doctor{Binary: binary, ConfigPath: "/etc/vps-agent/config.env", Timeout: 10 * time.Second}
}

func TestDoctorCheck_RunsTheDoctorSubcommand(t *testing.T) {
	d := doctor(script(t, `echo "$@"`))

	got := testkit.Must(d.Check(context.Background()))

	testkit.Equal(t, "doctor --config /etc/vps-agent/config.env\n", got)
}

func TestDoctorCheck_FailsWhenTheDoctorFails(t *testing.T) {
	d := doctor(script(t, "exit 1"))

	_, err := d.Check(context.Background())

	testkit.Equal(t, "doctor: exit status 1", err.Error())
}

func TestDoctorCheck_ReturnsTheOutputOfAFailedDoctor(t *testing.T) {
	d := doctor(script(t, "echo 'FAIL config: invalid config'; echo log >&2; exit 1"))

	got, _ := d.Check(context.Background())

	testkit.Equal(t, "FAIL config: invalid config\nlog\n", got)
}

func TestDoctorCheck_StopsADoctorThatHangs(t *testing.T) {
	d := doctor(script(t, "exec sleep 10"))
	d.Timeout = 50 * time.Millisecond

	_, err := d.Check(context.Background())

	testkit.Equal(t, "doctor: signal: killed", err.Error())
}

func TestDoctorCheck_FailsWhenTheBinaryIsMissing(t *testing.T) {
	d := doctor(filepath.Join(t.TempDir(), "vps-agent"))

	_, err := d.Check(context.Background())

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestExec_RunsTheCommand(t *testing.T) {
	err := process.Exec(context.Background(), "sh", "-c", "exit 0")

	testkit.ErrorIs(t, err, nil)
}

func TestExec_ReportsTheOutputOfAFailure(t *testing.T) {
	err := process.Exec(context.Background(), "sh", "-c", "echo nope; exit 3")

	testkit.Equal(t, "sh: exit status 3: nope", err.Error())
}
