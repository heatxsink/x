package paths

import (
	"os"
	"strings"
	"testing"
)

func TestNewCreatesDirectoriesWithStrictMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Force xdg to fall through to HOME-relative defaults.
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	p, err := New("paths-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for label, dir := range map[string]string{"LogPath": p.LogPath, "ConfigPath": p.ConfigPath} {
		if !strings.HasPrefix(dir, tmp) {
			t.Fatalf("%s = %q, want a path under tmp %q", label, dir, tmp)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s %q: %v", label, dir, err)
		}
		if mode := info.Mode().Perm(); mode != 0o700 {
			t.Errorf("%s %q mode = %o, want 0o700", label, dir, mode)
		}
	}
}

func TestNewPopulatesFilenames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	p, err := New("paths-test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p.LogFilename, "paths-test.log") {
		t.Errorf("LogFilename = %q, want suffix paths-test.log", p.LogFilename)
	}
	if !strings.HasSuffix(p.ConfigFilename, "paths-test.yaml") {
		t.Errorf("ConfigFilename = %q, want suffix paths-test.yaml", p.ConfigFilename)
	}
}
