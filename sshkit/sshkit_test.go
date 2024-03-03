package sshkit

import (
	"testing"
	"time"
)

var (
	hostname             = "sshkit.t.granado.io"
	username             = "alfred"
	port                 = 22
	privateKeyFilename   = ""
	privateKeyPassphrase = ""
	command              = "ls -alh /dev/tty"
	commandFail          = "lss -alh /dev/tty"
	client               *Client
)

func TestNew(t *testing.T) {
	var err error
	c := Config{
		Hostname:             hostname,
		Port:                 port,
		Username:             username,
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
		Hostname:             hostname,
		Port:                 port,
		Username:             username,
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
	c := Config{
		Hostname:             hostname,
		Port:                 port,
		Username:             username,
		Password:             "",
		PrivateKeyFilename:   privateKeyFilename,
		PrivateKeyPassphrase: privateKeyPassphrase,
	}
	client, err = New(&c)
	if err != nil {
		t.Error(err)
	}
	srcFilename := "/home/ngranado/go/src/github.com/heatxsink/hwebhookrouter/hwebhookrouter"
	destFilename := "/opt/hwebhookrouter/bin"
	err = client.Upload(srcFilename, destFilename, "0755", false)
	if err != nil {
		t.Error(err)
	}
}
