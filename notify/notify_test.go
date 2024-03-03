package notify

import (
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/exp/rand"
)

var (
	d *Discord
	p *Pushover
)

func init() {
	rand.Seed(uint64(time.Now().UnixNano()))
}

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestGet(t *testing.T) {
	path, err := os.UserHomeDir()
	if err != nil {
		t.Error(err)
	}
	d, err = GetDiscord("gir", path)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(d)
	p, err = GetPushover("piggy", path)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(p)
}

func TestDiscordMessage(t *testing.T) {
	message := randSeq(10)
	err := d.SendMessage(message)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(message)
}

func TestPushoverMessage(t *testing.T) {
	message := randSeq(10)
	err := p.SendMessage(message)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(message)
}

func TestPushoverGlance(t *testing.T) {
	title := randSeq(8)
	text := time.Now().Format(time.DateOnly)
	subText := time.Now().Format(time.TimeOnly)
	rr, err := p.SendGlance(title, text, subText, 0, 0)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(title, text, subText)
	if rr != nil {
		fmt.Println(rr.String())
		fmt.Println("Errors: ", rr.Errors)
	}
}
