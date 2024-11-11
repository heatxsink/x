package pushover

import (
	"bytes"
	"fmt"
	"io"
	"os"

	po "github.com/gregdel/pushover"
	"gopkg.in/yaml.v2"
)

type Pushover struct {
	Name       string `yaml:"name"`
	Enabled    bool   `yaml:"enabled"`
	AppToken   string `yaml:"app_token"`
	UserToken  string `yaml:"user_token"`
	DeviceName string `yaml:"device_name"`
}

func load(name string, service string, path string) ([]byte, error) {
	filename := fmt.Sprintf("%s/.hnotify.%s.%s.yaml", path, service, name)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func New(name string, path string) (*Pushover, error) {
	b, err := load(name, "pushover", path)
	if err != nil {
		return nil, err
	}
	var p Pushover
	err = yaml.Unmarshal(b, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Pushover) SendMessage(message string) error {
	if p.Enabled {
		app := po.New(p.AppToken)
		recipient := po.NewRecipient(p.UserToken)
		message := po.NewMessage(message)
		_, err := app.SendMessage(message, recipient)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Pushover) SendGlance(title string, text string, subText string, count int, percent int) (*po.Response, error) {
	var rr *po.Response
	var err error
	if p.Enabled {
		app := po.New(p.AppToken)
		r := po.NewRecipient(p.UserToken)
		g := &po.Glance{
			Title:      po.String(title),
			Text:       po.String(text),
			Subtext:    po.String(subText),
			Count:      po.Int(count),
			Percent:    po.Int(percent),
			DeviceName: po.GlancesAllDevices,
		}
		rr, err = app.SendGlanceUpdate(g, r)
		if err != nil {
			return nil, err
		}
	}
	return rr, nil
}

func (p *Pushover) SendGlanceTextOnly(title string, text string, subText string) (*po.Response, error) {
	var rr *po.Response
	var err error
	if p.Enabled {
		app := po.New(p.AppToken)
		r := po.NewRecipient(p.UserToken)
		g := &po.Glance{
			Title:      po.String(title),
			Text:       po.String(text),
			Subtext:    po.String(subText),
			DeviceName: po.GlancesAllDevices,
		}
		rr, err = app.SendGlanceUpdate(g, r)
		if err != nil {
			return nil, err
		}
	}
	return rr, nil
}

func (p *Pushover) SendGlanceCountOnly(count int) (*po.Response, error) {
	var rr *po.Response
	var err error
	if p.Enabled {
		app := po.New(p.AppToken)
		r := po.NewRecipient(p.UserToken)
		g := &po.Glance{
			Count:      po.Int(count),
			DeviceName: po.GlancesAllDevices,
		}
		rr, err = app.SendGlanceUpdate(g, r)
		if err != nil {
			return nil, err
		}
	}
	return rr, nil
}
