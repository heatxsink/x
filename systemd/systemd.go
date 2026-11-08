package systemd

import (
	"os"
	"text/template"
)

// serviceTemplate emits a single systemd unit file. It is rendered with
// text/template: html/template would entity-escape characters such as
// " & < + in ExecStart and corrupt the unit. Dependency and User= lines are
// emitted only when set, since an empty assignment resets the list in
// systemd rather than meaning "unset".
var serviceTemplate = `[Unit]
Description={{ .Name }}
{{- if .Wants }}
Wants={{ .Wants }}
{{- end }}
{{- if .After }}
After={{ .After }}
{{- end }}
{{- if .Requires }}
Requires={{ .Requires }}
{{- end }}

[Service]
{{- if .User }}
User={{ .User }}
{{- end }}
TimeoutStartSec={{ .TimeoutStartSec }}
ExecStart={{ .ExecStart }}
Restart={{ .Restart }}
RestartSec={{ .RestartSec }}

[Install]
WantedBy={{ .WantedBy }}
`

// Service is the data model for a generated systemd unit. Two
// presets:
//
//	NewService     -> system unit, runs as User (default "root"),
//	                  WantedBy=multi-user.target.
//	NewUserService -> user unit (lives under ~/.config/systemd/user/),
//	                  no User= line, WantedBy=default.target.
//
// Operators can also build a Service by hand for cases that don't
// fit either preset (e.g. a system unit running as a non-root user
// like "kiosk" -- start from NewService and overwrite .User).
type Service struct {
	Name            string
	Wants           string // weak dependency; pair "network-online.target" with After= to actually wait for the network
	After           string
	Requires        string
	User            string // empty => omit User= line (user services)
	ExecStart       string
	TimeoutStartSec int
	Restart         string
	RestartSec      int
	WantedBy        string // "multi-user.target" for system; "default.target" for user
}

// NewService returns a system-target unit running as root, with sane
// restart defaults. Override fields to change User, After, etc.
func NewService(name, execStart string) *Service {
	return &Service{
		Name:            name,
		User:            "root",
		ExecStart:       execStart,
		TimeoutStartSec: 0,
		Restart:         "always",
		RestartSec:      3,
		WantedBy:        "multi-user.target",
	}
}

// NewUserService returns a user-target unit (no User= line, installed
// under ~/.config/systemd/user/), with sane restart defaults.
// Operators run/manage these via `systemctl --user`.
func NewUserService(name, execStart string) *Service {
	return &Service{
		Name:            name,
		ExecStart:       execStart,
		TimeoutStartSec: 0,
		Restart:         "always",
		RestartSec:      3,
		WantedBy:        "default.target",
	}
}

func (s *Service) ToFile(filename string) error {
	tmpl, err := template.New(s.Name).Parse(serviceTemplate)
	if err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return tmpl.Execute(f, s)
}
