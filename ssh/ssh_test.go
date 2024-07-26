package ssh

import (
	"testing"
	"time"
)

var (
	hostname             = "sk.t.granado.io"
	username             = "alfred"
	port                 = 22
	password             = "1234"
	privateKeyFilename   = "./abc"
	privateKeyPassphrase = "4321"
	client               *Client
	pkClient             *Client
)

func TestNewWithPassword(t *testing.T) {
	var err error
	client, err = NewWithPassword(hostname, port, username, password)
	if err != nil {
		t.Error(err)
	}
	client.SetProperty("PubkeyAuthentication", "no")
	client.ClientConfig.Timeout = 10 * time.Second
}

func TestExecute(t *testing.T) {
	err := client.Execute("ls -alh /dev/tty")
	if err != nil {
		t.Error(err)
	}
}

func TestExecuteInteractively(t *testing.T) {
	err := client.ExecuteInteractively("ls -alh /dev/tty", map[string]string{
		"Password:": "1234",
	})
	if err != nil {
		t.Error(err)
	}
}

func TestKeyFail(t *testing.T) {
	var err error
	pkClient, err = NewWithPrivateKey(hostname, port, username, privateKeyFilename, privateKeyPassphrase)
	if err != nil {
		t.Error(err)
	}
}

func TestUpload(t *testing.T) {
	srcFilename := "/home/ngranado/go/src/github.com/heatxsink/hwebhookrouter/hwebhookrouter"
	destFilename := "/opt/hwebhookrouter/bin"
	err := pkClient.Upload(srcFilename, destFilename, "0755", false)
	if err != nil {
		t.Error(err)
	}
}
