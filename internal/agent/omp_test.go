package agent

import (
	"path/filepath"
	"testing"
)

func TestOMPValidSessionHomeShorthand(t *testing.T) {
	home := t.TempDir()
	// cwd == home resolves to the "-" shorthand directory under the XDG
	// sessions root, per omp_session_dir_names()'s primary-name rule.
	sessionDir := filepath.Join(home, ".local", "share", "omp", "sessions", "-")
	writeJSONLSession(t, filepath.Join(sessionDir, "session.jsonl"), "home-session-id", home)

	o := OMP{Home: home}

	if !o.ValidSession("home-session-id", home) {
		t.Error("session under the home-shorthand dir should be valid")
	}
	if o.ValidSession("no-such-id", home) {
		t.Error("nonexistent session should not be valid")
	}

	newest, ok := o.NewestSession(home)
	if !ok || newest != "home-session-id" {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", newest, ok, "home-session-id")
	}
}

func TestOMPValidSessionLegacyDir(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "some", "project")
	// The legacy "--full-path--" directory form must also be found, since
	// sessionDirsFor unions primary + legacy names.
	legacyDir := filepath.Join(home, ".local", "share", "omp", "sessions", sanitizePathComponent(cwd))
	legacyDir = filepath.Join(filepath.Dir(legacyDir), "--"+sanitizePathComponent(cwd)+"--")
	writeJSONLSession(t, filepath.Join(legacyDir, "session.jsonl"), "legacy-session-id", cwd)

	o := OMP{Home: home}
	if !o.ValidSession("legacy-session-id", cwd) {
		t.Error("session under the legacy full-path dir should be valid")
	}
}

func TestOMPMatchesExcludesWorkers(t *testing.T) {
	o := OMP{}
	cases := map[string]bool{
		"omp --resume abc":             true,
		"/usr/bin/omp":                 true,
		"omp __omp_worker_1 something": false,
	}
	for cmdline, want := range cases {
		if got := o.Matches(cmdline); got != want {
			t.Errorf("Matches(%q) = %v, want %v", cmdline, got, want)
		}
	}
}
