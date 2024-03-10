package systemd

import (
	"html/template"
	"os"
)

var serviceTemplate = `[Unit]
Description={{ .Name }}
After={{ .After }}
Requires={{ .Requires }}

[Service]
user={{ .User }}
TimeoutStartSec={{ .TimeoutStartSec }}
ExecStart={{ .ExecStart }}
Restart={{ .Restart }}
RestartSec={{ .RestartSec }}

[Install]
WantedBy=multi-user.target
`

type Service struct {
	Name            string // name service
	After           string //
	Requires        string //
	User            string // root
	ExecStart       string // command
	TimeoutStartSec int    //0
	Restart         string //always
	RestartSec      int    //3
}

func NewService(name, execStart string) *Service {
	return &Service{
		Name:            name,
		After:           "",
		Requires:        "",
		User:            "root",
		ExecStart:       execStart,
		TimeoutStartSec: 0,
		Restart:         "always",
		RestartSec:      3,
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
	defer f.Close()
	return tmpl.Execute(f, s)
}
