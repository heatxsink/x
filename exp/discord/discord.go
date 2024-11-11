package discord

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/heatxsink/x/webhook"
	"gopkg.in/yaml.v2"
)

type Discord struct {
	Name       string `yaml:"name"`
	Enabled    bool   `yaml:"enabled"`
	Username   string `yaml:"username"`
	WebhookURL string `yaml:"webhook_url"`
}

type Payload struct {
	Username string          `json:"username,omitempty"`
	Content  string          `json:"content,omitempty"`
	Embeds   []*MessageEmbed `json:"embeds,omitempty"`
}

type MessageEmbed struct {
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	Color       int                  `json:"color,omitempty"`
	Fields      []*MessageEmbedField `json:"fields,omitempty"`
}

type MessageEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
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

func New(name string, path string) (*Discord, error) {
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

func (d *Discord) SendContent(content string) error {
	if d.Enabled {
		var dp Payload
		dp.Username = d.Username
		dp.Content = content
		return webhook.SendJSON(d.WebhookURL, &dp)
	}
	return nil
}

func (d *Discord) SendEmbeds(embeds []*MessageEmbed) error {
	if d.Enabled {
		var dp Payload
		dp.Username = d.Username
		dp.Embeds = embeds
		return webhook.SendJSON(d.WebhookURL, &dp)
	}
	return nil
}
