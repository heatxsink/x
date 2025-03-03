package ssh

import (
	"testing"
	"time"
)

var (
	hostname = "gaz.h.granado.io"
	port     = 22
	username = "pi"
	password = "1234"
)

func TestExecute(t *testing.T) {
	client, err := NewWithPassword(hostname, port, username, password)
	if err != nil {
		t.Error(err)
	}
	client.SetProperty("PubkeyAuthentication", "no")
	client.ClientConfig.Timeout = 10 * time.Second
	err = client.Execute("ls -alh /dev/tty")
	if err != nil {
		t.Error(err)
	}
}

func TestExecuteInteractively(t *testing.T) {
	client, err := NewWithPassword(hostname, port, username, password)
	if err != nil {
		t.Error(err)
	}
	client.SetProperty("PubkeyAuthentication", "no")
	client.ClientConfig.Timeout = 10 * time.Second
	err = client.ExecuteInteractively("ls -alh /dev/tty", map[string]string{
		"Password:": password,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestUpload(t *testing.T) {
	srcFilename := "/Users/ngranado/go/src/github.com/heatxsink/hlights/hlights"
	destFilename := "/home/pi/hlights"
	client, err := NewWithPassword(hostname, 22, username, password)
	if err != nil {
		t.Error(err)
	}
	client.SetProperty("PubkeyAuthentication", "no")
	client.ClientConfig.Timeout = 10 * time.Second
	err = client.Upload(srcFilename, destFilename, "0755", false)
	if err != nil {
		t.Error(err)
	}
}
