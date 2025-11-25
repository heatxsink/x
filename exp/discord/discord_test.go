package discord

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

var (
	d *Discord
)

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestInit(t *testing.T) {
	path, err := os.UserHomeDir()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(path)
	d, err = New("gir", path)
	if err != nil {
		t.Skip("Skipping test: config file not available -", err)
	}
	fmt.Println(d)
}

func TestContent(t *testing.T) {
	if d == nil {
		t.Skip("Skipping test: Discord client not initialized")
	}
	message := randSeq(10)
	err := d.SendContent(message)
	if err != nil {
		t.Error(err)
	}
}

func TestEmbeds(t *testing.T) {
	if d == nil {
		t.Skip("Skipping test: Discord client not initialized")
	}
	embed := &MessageEmbed{
		Title:       "Unit Test",
		Description: "This is a unit test.",
		Color:       15512110,
		Fields: []*MessageEmbedField{
			{
				Name:   "Who's Playing",
				Value:  "This is a test.",
				Inline: false,
			},
			{
				Name:   "Start Time",
				Value:  time.Now().Format(time.RFC3339),
				Inline: false,
			},
		},
	}
	var ee []*MessageEmbed
	ee = append(ee, embed)
	err := d.SendEmbeds(ee)
	if err != nil {
		t.Error(err)
	}
}
