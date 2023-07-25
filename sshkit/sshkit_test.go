package sshkit

import (
	"testing"
	"time"

	"github.com/heatxsink/exp/termkit"
)

var (
	command     = "ls -alh /dev/tty"
	commandFail = "lss -alh /dev/tty"
	client      *Client
)

func TestNew(t *testing.T) {
	var err error
	c := Config{
		Hostname:             "ssmaster.h.granado.io",
		Port:                 22,
		Username:             "ngranado",
		Password:             "1234",
		PrivateKeyFilename:   "",
		PrivateKeyPassphrase: "",
	}
	client, err = New(&c)
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
		"Password:": client.config.Password,
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
		"Password:": client.config.Password,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestKeyFail(t *testing.T) {
	var err error
	c := Config{
		Hostname:             "ssmaster.h.granado.io",
		Port:                 22,
		Username:             "ngranado",
		Password:             "",
		PrivateKeyFilename:   "./abc",
		PrivateKeyPassphrase: "",
	}
	client, err = New(&c)
	if err != nil {
		t.Error(err)
	}
}

func TestUpload(t *testing.T) {
	var err error
	passphrase := termkit.PasswordPrompt("Enter passphrase:")
	c := Config{
		Hostname:             "143.198.104.64",
		Port:                 22,
		Username:             "root",
		Password:             "",
		PrivateKeyFilename:   "/home/ngranado/.ssh/keys/personal/digitalocean",
		PrivateKeyPassphrase: passphrase,
	}
	client, err = New(&c)
	if err != nil {
		t.Error(err)
	}
	err = client.Upload("/home/ngranado/go/src/github.com/heatxsink/hwebhookrouter/hwebhookrouter", "/root/hwebhookrouter", "0755", true)
	if err != nil {
		t.Error(err)
	}
}
