package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/gregdel/pushover"
	"gopkg.in/yaml.v2"
)

// Discord struct
type Discord struct {
	Name       string `yaml:"name"`
	Enabled    bool   `yaml:"enabled"`
	Username   string `yaml:"username"`
	WebhookURL string `yaml:"webhook_url"`
}

// Pushover struct
type Pushover struct {
	Name       string `yaml:"name"`
	Enabled    bool   `yaml:"enabled"`
	AppToken   string `yaml:"app_token"`
	UserToken  string `yaml:"user_token"`
	DeviceName string `yaml:"device_name"`
}

func GetDiscord(name string, path string) (*Discord, error) {
	b, err := load(name, "discord", path)
	if err != nil {
		return nil, err
	}
	var d Discord
	err = yaml.Unmarshal(b, &d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func GetPushover(name string, path string) (*Pushover, error) {
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

func save(name string, service string, path string, object interface{}) error {
	data, err := yaml.Marshal(&object)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("%s/.hnotify.%s.%s.yaml", path, service, name)
	return os.WriteFile(filename, data, 0644)
}

func httpPost(url string, payload []byte) ([]byte, error) {
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (p *Pushover) SendMessage(message string) error {
	if p.Enabled {
		app := pushover.New(p.AppToken)
		recipient := pushover.NewRecipient(p.UserToken)
		message := pushover.NewMessage(message)
		_, err := app.SendMessage(message, recipient)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Pushover) SendGlance(title string, text string, subText string, count int, percent int) (*pushover.Response, error) {
	var rr *pushover.Response
	var err error
	if p.Enabled {
		app := pushover.New(p.AppToken)
		r := pushover.NewRecipient(p.UserToken)
		g := &pushover.Glance{
			Title:      pushover.String(title),
			Text:       pushover.String(text),
			Subtext:    pushover.String(subText),
			Count:      pushover.Int(count),
			Percent:    pushover.Int(percent),
			DeviceName: p.DeviceName,
		}
		rr, err = app.SendGlanceUpdate(g, r)
		if err != nil {
			return nil, err
		}
	}
	return rr, nil
}

func (d *Discord) SendMessage(message string) error {
	if d.Enabled {
		payload := fmt.Sprintf("{\"username\": \"%s\", \"content\": \"%s\"}", d.Username, message)
		_, err := httpPost(d.WebhookURL, []byte(payload))
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Discord) SendMessageEmbed(embed *discordgo.MessageEmbed) error {
	if d.Enabled {
		ej, err := json.Marshal(embed)
		if err != nil {
			return err
		}
		payload := fmt.Sprintf("{\"embeds\": [%s]}", string(ej))
		_, err = httpPost(d.WebhookURL, []byte(payload))
		if err != nil {
			return err
		}
	}
	return nil
}
