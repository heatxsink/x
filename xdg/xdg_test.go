package xdg

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewScope(t *testing.T) {
	s := NewScope(User, "myapp")
	if s.Type != User || s.App != "myapp" {
		t.Fatalf("unexpected scope: %+v", s)
	}
}

func TestNewVendorScope(t *testing.T) {
	s := NewVendorScope(User, "myvendor", "myapp")
	if s.Vendor != "myvendor" || s.App != "myapp" {
		t.Fatalf("unexpected scope: %+v", s)
	}
}

func TestNewCustomHomeScope(t *testing.T) {
	s := NewCustomHomeScope("/tmp/home", "vendor", "app")
	if s.Type != CustomHome || s.CustomHome != "/tmp/home" {
		t.Fatalf("unexpected scope: %+v", s)
	}
}

func TestAppPath(t *testing.T) {
	s := NewScope(User, "myapp")
	if s.appPath() != "myapp" {
		t.Errorf("appPath = %q, want %q", s.appPath(), "myapp")
	}

	s = NewVendorScope(User, "vendor", "myapp")
	expected := filepath.Join("vendor", "myapp")
	if s.appPath() != expected {
		t.Errorf("appPath = %q, want %q", s.appPath(), expected)
	}
}

func TestUserConfigPath(t *testing.T) {
	s := NewScope(User, "testapp")
	p, err := s.ConfigPath("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, "testapp") {
		t.Errorf("config path should contain app name: %s", p)
	}
	if !strings.HasSuffix(p, "config.yaml") {
		t.Errorf("config path should end with filename: %s", p)
	}
}

func TestUserDataPath(t *testing.T) {
	s := NewScope(User, "testapp")
	p, err := s.DataPath("data.db")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, "testapp") {
		t.Errorf("data path should contain app name: %s", p)
	}
	if !strings.HasSuffix(p, "data.db") {
		t.Errorf("data path should end with filename: %s", p)
	}
}

func TestUserLogPath(t *testing.T) {
	s := NewScope(User, "testapp")
	p, err := s.LogPath("app.log")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, "testapp") {
		t.Errorf("log path should contain app name: %s", p)
	}
	if !strings.HasSuffix(p, "app.log") {
		t.Errorf("log path should end with filename: %s", p)
	}
}

func TestUserCacheDir(t *testing.T) {
	s := NewScope(User, "testapp")
	d, err := s.CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "testapp") {
		t.Errorf("cache dir should contain app name: %s", d)
	}
}

func TestSystemPaths(t *testing.T) {
	s := NewScope(System, "testapp")

	dataDirs, err := s.DataDirs()
	if err != nil {
		t.Fatal(err)
	}
	if len(dataDirs) == 0 {
		t.Error("system data dirs should not be empty")
	}

	configDirs, err := s.ConfigDirs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configDirs) == 0 {
		t.Error("system config dirs should not be empty")
	}
}

func TestCustomHomeScope(t *testing.T) {
	dir := t.TempDir()
	s := NewCustomHomeScope(dir, "", "testapp")

	p, err := s.ConfigPath("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p, dir) {
		t.Errorf("custom home path should start with %s: %s", dir, p)
	}
	if !strings.Contains(p, "testapp") {
		t.Errorf("custom home path should contain app name: %s", p)
	}
}

func TestCustomHomeScopeEmptyPath(t *testing.T) {
	s := NewCustomHomeScope("", "", "testapp")
	_, err := s.ConfigPath("config.yaml")
	if err == nil {
		t.Error("expected error for empty custom home")
	}
}

func TestInvalidScopeType(t *testing.T) {
	s := &Scope{Type: ScopeType(99), App: "testapp"}
	_, err := s.ConfigPath("config.yaml")
	if err == nil {
		t.Error("expected error for invalid scope type")
	}
}

func TestXDGEnvOverride(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG env vars only apply on Linux/Unix")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	s := NewScope(User, "testapp")
	p, err := s.ConfigPath("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, "testapp", "config.yaml")
	if p != expected {
		t.Errorf("config path = %q, want %q", p, expected)
	}
}

func TestXDGDataDirsEnvOverride(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG env vars only apply on Linux/Unix")
	}
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir1)
	t.Setenv("XDG_DATA_DIRS", dir2)

	s := NewScope(User, "testapp")
	dirs, err := s.DataDirs()
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) < 2 {
		t.Fatalf("expected at least 2 dirs, got %d", len(dirs))
	}
	if dirs[0] != filepath.Join(dir1, "testapp") {
		t.Errorf("first dir = %q, want %q", dirs[0], filepath.Join(dir1, "testapp"))
	}
	if dirs[1] != filepath.Join(dir2, "testapp") {
		t.Errorf("second dir = %q, want %q", dirs[1], filepath.Join(dir2, "testapp"))
	}
}

func TestLookupConfigFindsExistingFile(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG env vars only apply on Linux/Unix")
	}
	dir := t.TempDir()
	appDir := filepath.Join(dir, "testapp")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "config.yaml"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", dir)

	s := NewScope(User, "testapp")
	found, err := s.LookupConfig("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Error("expected to find config file")
	}
}

func TestLookupConfigNoMatch(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG env vars only apply on Linux/Unix")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CONFIG_DIRS", dir)

	s := NewScope(User, "testapp")
	found, err := s.LookupConfig("nonexistent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("expected no matches, got %d", len(found))
	}
}

func TestVendorInPath(t *testing.T) {
	s := NewVendorScope(User, "myvendor", "myapp")
	p, err := s.ConfigPath("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, filepath.Join("myvendor", "myapp")) {
		t.Errorf("path should contain vendor/app: %s", p)
	}
}

func TestDataDirsNotEmpty(t *testing.T) {
	for _, st := range []ScopeType{User, System} {
		s := NewScope(st, "testapp")
		dirs, err := s.DataDirs()
		if err != nil {
			t.Fatalf("scope %d: %v", st, err)
		}
		if len(dirs) == 0 {
			t.Errorf("scope %d: data dirs should not be empty", st)
		}
	}
}

func TestConfigDirsNotEmpty(t *testing.T) {
	for _, st := range []ScopeType{User, System} {
		s := NewScope(st, "testapp")
		dirs, err := s.ConfigDirs()
		if err != nil {
			t.Fatalf("scope %d: %v", st, err)
		}
		if len(dirs) == 0 {
			t.Errorf("scope %d: config dirs should not be empty", st)
		}
	}
}
