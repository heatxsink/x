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

type Option func(*Pushover) error

func load(name string, service string, path string) ([]byte, error) {
	filename := fmt.Sprintf("%s/.hnotify.%s.%s.yaml", path, service, name)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func New(name string, opts ...Option) (*Pushover, error) {
	p := &Pushover{
		Name:    name,
		Enabled: true,
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	if p.AppToken == "" || p.UserToken == "" {
		return nil, fmt.Errorf("pushover: appToken and userToken are required")
	}
	return p, nil
}

func WithConfigFile(path string) Option {
	return func(p *Pushover) error {
		b, err := load(p.Name, "pushover", path)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(b, p)
	}
}

func WithTokens(appToken, userToken string) Option {
	return func(p *Pushover) error {
		p.AppToken = appToken
		p.UserToken = userToken
		return nil
	}
}

func WithDeviceName(deviceName string) Option {
	return func(p *Pushover) error {
		p.DeviceName = deviceName
		return nil
	}
}

func (p *Pushover) SendMessage(message string) error {
	if p.Enabled {
		app := po.New(p.AppToken)
		recipient := po.NewRecipient(p.UserToken)
		msg := po.NewMessage(message)
		_, err := app.SendMessage(msg, recipient)
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
