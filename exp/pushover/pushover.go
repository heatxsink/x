package pushover

import (
	"fmt"
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

func load(name, service, path string) ([]byte, error) {
	filename := fmt.Sprintf("%s/.hnotify.%s.%s.yaml", path, service, name)
	return os.ReadFile(filename)
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
	if !p.Enabled {
		return nil
	}
	app := po.New(p.AppToken)
	recipient := po.NewRecipient(p.UserToken)
	msg := po.NewMessage(message)
	_, err := app.SendMessage(msg, recipient)
	return err
}

func (p *Pushover) SendGlance(title, text, subText string, count, percent int) (*po.Response, error) {
	if !p.Enabled {
		return nil, nil
	}
	app := po.New(p.AppToken)
	r := po.NewRecipient(p.UserToken)
	g := &po.Glance{
		Title:      po.String(title),
		Text:       po.String(text),
		Subtext:    po.String(subText),
		Count:      po.Int(count),
		Percent:    po.Int(percent),
		DeviceName: p.glanceDeviceName(),
	}
	return app.SendGlanceUpdate(g, r)
}

func (p *Pushover) SendGlanceTextOnly(title, text, subText string) (*po.Response, error) {
	if !p.Enabled {
		return nil, nil
	}
	app := po.New(p.AppToken)
	r := po.NewRecipient(p.UserToken)
	g := &po.Glance{
		Title:      po.String(title),
		Text:       po.String(text),
		Subtext:    po.String(subText),
		DeviceName: p.glanceDeviceName(),
	}
	return app.SendGlanceUpdate(g, r)
}

func (p *Pushover) SendGlanceCountOnly(count int) (*po.Response, error) {
	if !p.Enabled {
		return nil, nil
	}
	app := po.New(p.AppToken)
	r := po.NewRecipient(p.UserToken)
	g := &po.Glance{
		Count:      po.Int(count),
		DeviceName: p.glanceDeviceName(),
	}
	return app.SendGlanceUpdate(g, r)
}

func (p *Pushover) glanceDeviceName() string {
	if p.DeviceName != "" {
		return p.DeviceName
	}
	return po.GlancesAllDevices
}
