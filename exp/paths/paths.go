package paths

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	gap "github.com/muesli/go-app-paths"
)

type Paths struct {
	LogFilename    string
	LogPath        string
	ConfigFilename string
	ConfigPath     string
}

func New(name string) (*Paths, error) {
	p := Paths{}
	var err error
	scope := gap.NewScope(gap.User, name)
	logFilename := fmt.Sprintf("%s.log", name)
	p.LogFilename, err = scope.LogPath(logFilename)
	if err != nil {
		return nil, err
	}
	p.LogPath = filepath.Dir(p.LogFilename)
	err = os.MkdirAll(p.LogPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	configFilename := fmt.Sprintf("%s.yaml", name)
	p.ConfigFilename, err = scope.ConfigPath(configFilename)
	if err != nil {
		return nil, err
	}
	p.ConfigPath = filepath.Dir(p.ConfigFilename)
	err = os.MkdirAll(p.ConfigPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Paths) String() string {
	b := new(bytes.Buffer)
	b.WriteString("Paths:  \n")
	fmt.Fprintf(b, "  LogFilename:    %v\n", p.LogFilename)
	fmt.Fprintf(b, "  LogPath:        %v\n", p.LogPath)
	fmt.Fprintf(b, "  ConfigFilename: %v\n", p.ConfigFilename)
	fmt.Fprintf(b, "  ConfigPath:     %v\n", p.ConfigPath)
	b.WriteString("\n")
	return b.String()
}
