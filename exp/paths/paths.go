package paths

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	gap "github.com/muesli/go-app-paths"
)

type Paths struct {
	Log    string
	Config string
}

func New(name string) (*Paths, error) {
	p := Paths{}
	var err error
	scope := gap.NewScope(gap.User, name)
	logFilename := fmt.Sprintf("%s.log", name)
	p.Log, err = scope.LogPath(logFilename)
	if err != nil {
		return nil, err
	}
	lp := filepath.Dir(p.Log)
	err = os.MkdirAll(lp, os.ModePerm)
	if err != nil {
		return nil, err
	}
	configFilename := fmt.Sprintf("%s.yaml", name)
	p.Config, err = scope.ConfigPath(configFilename)
	if err != nil {
		return nil, err
	}
	cp := filepath.Dir(p.Config)
	err = os.MkdirAll(cp, os.ModePerm)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Paths) String() string {
	b := new(bytes.Buffer)
	b.WriteString("Paths:  \n")
	fmt.Fprintf(b, "  Log:     %v\n", p.Log)
	fmt.Fprintf(b, "  Config:  %v\n", p.Config)
	b.WriteString("\n")
	return b.String()
}
