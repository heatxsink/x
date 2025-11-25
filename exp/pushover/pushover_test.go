package pushover

import (
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/exp/rand"
)

var (
	p *Pushover
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
	p, err = New("piggy", path)
	if err != nil {
		t.Skip("Skipping test: config file not available -", err)
	}
	fmt.Println(p)
}

func TestMessage(t *testing.T) {
	if p == nil {
		t.Skip("Skipping test: Pushover client not initialized")
	}
	message := randSeq(10)
	err := p.SendMessage(message)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(message)
}

func TestGlance(t *testing.T) {
	if p == nil {
		t.Skip("Skipping test: Pushover client not initialized")
	}
	title := randSeq(8)
	text := time.Now().Format(time.DateOnly)
	subText := time.Now().Format(time.TimeOnly)
	rr, err := p.SendGlance(title, text, subText, 0, 0)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(title, text, subText)
	fmt.Println(rr.String())
}
