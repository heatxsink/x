package sshkit

import (
	"testing"
	"time"
)

var (
	hostname    = "ssmaster.h.granado.io"
	port        = "22"
	username    = "ngranado"
	password    = "1234"
	key         = ""
	command     = "ls -alh /dev/tty"
	commandFail = "lss -alh /dev/tty"
	client      *Client
)

func TestNew(t *testing.T) {
	var err error
	client, err = New(hostname, port, username, password, key, "")
	if err != nil {
		t.Error(err)
	}
	client.SetProperty("PubkeyAuthentication", "no")
	client.ClientConfig.Timeout = 10 * time.Second
}

func TestExecute(t *testing.T) {
	err := client.Execute(command)
	if err != nil {
		t.Error(err)
	}
}

func TestExecuteInteractively(t *testing.T) {
	err := client.ExecuteInteractively(command, map[string]string{
		"Password:": password,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestExecuteFail(t *testing.T) {
	err := client.Execute(commandFail)
	if err != nil {
		t.Error(err)
	}
}

func TestExecuteInteractivelyFail(t *testing.T) {
	err := client.ExecuteInteractively(commandFail, map[string]string{
		"Password:": password,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestKeyFail(t *testing.T) {
	var err error
	client, err = New(hostname, port, username, password, "./abc", "")
	if err != nil {
		t.Error(err)
	}
}

func TestUpload(t *testing.T) {
	var err error
	client, err = New("143.198.104.64", "22", "root", "", "/home/ngranado/.ssh/keys/personal/digitalocean", "nick2360")
	if err != nil {
		t.Error(err)
	}
	err = client.Upload("/home/ngranado/go/src/github.com/heatxsink/hwebhookrouter/hwebhookrouter", "/root/hwebhookrouter", "0755", true)
	if err != nil {
		t.Error(err)
	}
}
