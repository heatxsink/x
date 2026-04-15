package xdg

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrInvalidScope   = errors.New("xdg: invalid scope type")
	ErrRetrievingPath = errors.New("xdg: could not retrieve path")
)

// ScopeType determines whether paths resolve to user or system locations.
type ScopeType int

const (
	System     ScopeType = iota // System-wide paths
	User                        // Per-user paths
	CustomHome                  // Custom home directory
)

// Scope holds the context for resolving application paths.
type Scope struct {
	Type       ScopeType
	CustomHome string
	Vendor     string
	App        string
}

// NewScope creates a new Scope for the given type and app name.
func NewScope(t ScopeType, app string) *Scope {
	return &Scope{Type: t, App: app}
}

// NewVendorScope creates a new Scope with a vendor prefix.
func NewVendorScope(t ScopeType, vendor, app string) *Scope {
	return &Scope{Type: t, Vendor: vendor, App: app}
}

// NewCustomHomeScope creates a scope rooted at a custom home directory.
func NewCustomHomeScope(path, vendor, app string) *Scope {
	return &Scope{Type: CustomHome, CustomHome: path, Vendor: vendor, App: app}
}

func (s *Scope) appPath(parts ...string) string {
	if s.Vendor != "" {
		return filepath.Join(append([]string{s.Vendor, s.App}, parts...)...)
	}
	return filepath.Join(append([]string{s.App}, parts...)...)
}

// DataDirs returns the list of directories to search for data files.
func (s *Scope) DataDirs() ([]string, error) {
	base, err := s.dataDirs()
	if err != nil {
		return nil, err
	}
	result := make([]string, len(base))
	for i, d := range base {
		result[i] = filepath.Join(d, s.appPath())
	}
	return result, nil
}

// ConfigDirs returns the list of directories to search for config files.
func (s *Scope) ConfigDirs() ([]string, error) {
	base, err := s.configDirs()
	if err != nil {
		return nil, err
	}
	result := make([]string, len(base))
	for i, d := range base {
		result[i] = filepath.Join(d, s.appPath())
	}
	return result, nil
}

// CacheDir returns the base cache directory for the application.
func (s *Scope) CacheDir() (string, error) {
	base, err := s.cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, s.appPath()), nil
}

// DataPath returns the full path for a data file.
func (s *Scope) DataPath(filename string) (string, error) {
	dirs, err := s.DataDirs()
	if err != nil {
		return "", err
	}
	if len(dirs) == 0 {
		return "", ErrRetrievingPath
	}
	return filepath.Join(dirs[0], filename), nil
}

// ConfigPath returns the full path for a config file.
func (s *Scope) ConfigPath(filename string) (string, error) {
	dirs, err := s.ConfigDirs()
	if err != nil {
		return "", err
	}
	if len(dirs) == 0 {
		return "", ErrRetrievingPath
	}
	return filepath.Join(dirs[0], filename), nil
}

// LogPath returns the full path for a log file.
func (s *Scope) LogPath(filename string) (string, error) {
	base, err := s.logDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, s.appPath(), filename), nil
}

// LookupConfig returns paths to existing config files with the given name.
func (s *Scope) LookupConfig(filename string) ([]string, error) {
	dirs, err := s.ConfigDirs()
	if err != nil {
		return nil, err
	}
	return lookupFile(dirs, filename), nil
}

// LookupDataFile returns paths to existing data files with the given name.
func (s *Scope) LookupDataFile(filename string) ([]string, error) {
	dirs, err := s.DataDirs()
	if err != nil {
		return nil, err
	}
	return lookupFile(dirs, filename), nil
}

func lookupFile(dirs []string, filename string) []string {
	var found []string
	for _, d := range dirs {
		p := filepath.Join(d, filename)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return found
}

// userHomeDir returns the current user's home directory. It tries
// os.UserHomeDir first ($HOME), then falls back to os/user.Current
// which reads /etc/passwd. This ensures correct behavior in environments
// where $HOME is not set, such as systemd services.
func userHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		return home, nil
	}
	u, uerr := user.Current()
	if uerr != nil {
		return "", err
	}
	return u.HomeDir, nil
}

func homeDir(s *Scope) (string, error) {
	switch s.Type {
	case CustomHome:
		if s.CustomHome == "" {
			return "", ErrInvalidScope
		}
		return s.CustomHome, nil
	case User:
		return userHomeDir()
	case System:
		return "", nil
	}
	return "", ErrInvalidScope
}

func (s *Scope) dataDirs() ([]string, error) {
	switch runtime.GOOS {
	case "darwin":
		return s.darwinDirs("Application Support")
	case "windows":
		return s.windowsDirs("")
	default:
		return s.xdgDirs("XDG_DATA_HOME", ".local/share", "XDG_DATA_DIRS", "/usr/local/share:/usr/share")
	}
}

func (s *Scope) configDirs() ([]string, error) {
	switch runtime.GOOS {
	case "darwin":
		return s.darwinDirs("Preferences")
	case "windows":
		return s.windowsDirs("Config")
	default:
		return s.xdgDirs("XDG_CONFIG_HOME", ".config", "XDG_CONFIG_DIRS", "/etc/xdg:/etc")
	}
}

func (s *Scope) cacheDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return s.darwinDir("Caches")
	case "windows":
		return s.windowsDir("Cache")
	default:
		return s.xdgDir("XDG_CACHE_HOME", ".cache", "/var/cache")
	}
}

func (s *Scope) logDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return s.darwinDir("Logs")
	case "windows":
		return s.windowsDir("Logs")
	default:
		return s.xdgDir("", ".local/share", "/var/log")
	}
}

// xdgDir returns a single directory based on XDG env var or defaults.
func (s *Scope) xdgDir(envKey, userDefault, systemDefault string) (string, error) {
	switch s.Type {
	case User:
		if envKey != "" {
			if v := os.Getenv(envKey); v != "" {
				return v, nil
			}
		}
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, userDefault), nil
	case System:
		return systemDefault, nil
	case CustomHome:
		if s.CustomHome == "" {
			return "", ErrInvalidScope
		}
		return filepath.Join(s.CustomHome, userDefault), nil
	}
	return "", ErrInvalidScope
}

// xdgDirs returns multiple directories: primary + fallback list.
func (s *Scope) xdgDirs(homeEnv, homeDefault, dirsEnv, dirsDefault string) ([]string, error) {
	switch s.Type {
	case User:
		primary := os.Getenv(homeEnv)
		if primary == "" {
			home, err := userHomeDir()
			if err != nil {
				return nil, err
			}
			primary = filepath.Join(home, homeDefault)
		}
		dirs := []string{primary}
		extra := os.Getenv(dirsEnv)
		if extra == "" {
			extra = dirsDefault
		}
		dirs = append(dirs, splitPaths(extra)...)
		return dirs, nil
	case System:
		extra := os.Getenv(dirsEnv)
		if extra == "" {
			extra = dirsDefault
		}
		return splitPaths(extra), nil
	case CustomHome:
		if s.CustomHome == "" {
			return nil, ErrInvalidScope
		}
		return []string{filepath.Join(s.CustomHome, homeDefault)}, nil
	}
	return nil, ErrInvalidScope
}

func (s *Scope) darwinDir(subdir string) (string, error) {
	home, err := homeDir(s)
	if err != nil {
		return "", err
	}
	switch s.Type {
	case User, CustomHome:
		return filepath.Join(home, "Library", subdir), nil
	case System:
		return "/Library/" + subdir, nil
	}
	return "", ErrInvalidScope
}

func (s *Scope) darwinDirs(subdir string) ([]string, error) {
	d, err := s.darwinDir(subdir)
	if err != nil {
		return nil, err
	}
	dirs := []string{d}
	if s.Type == User {
		dirs = append(dirs, "/Library/"+subdir)
	}
	return dirs, nil
}

func (s *Scope) windowsDir(subdir string) (string, error) {
	base, err := s.windowsBase()
	if err != nil {
		return "", err
	}
	if subdir == "" {
		return base, nil
	}
	return filepath.Join(base, subdir), nil
}

func (s *Scope) windowsDirs(subdir string) ([]string, error) {
	d, err := s.windowsDir(subdir)
	if err != nil {
		return nil, err
	}
	return []string{d}, nil
}

func (s *Scope) windowsBase() (string, error) {
	switch s.Type {
	case User:
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return v, nil
		}
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Local"), nil
	case System:
		if v := os.Getenv("PROGRAMDATA"); v != "" {
			return v, nil
		}
		return `C:\ProgramData`, nil
	case CustomHome:
		if s.CustomHome == "" {
			return "", ErrInvalidScope
		}
		return s.CustomHome, nil
	}
	return "", ErrInvalidScope
}

func splitPaths(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, string(os.PathListSeparator))
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
