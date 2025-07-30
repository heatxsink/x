package loom

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/heatxsink/x/ssh"
	"github.com/heatxsink/x/systemd"
	"github.com/joho/godotenv"
)

type Loom struct {
	login       string
	password    string
	useAgent    bool
	hostname    string
	port        string
	destination string
	serviceName string
}

func New(serviceName string, useAgent bool) (*Loom, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}
	return &Loom{
		serviceName: serviceName,
		useAgent:    useAgent,
		login:       os.Getenv("LOOM_SSH_LOGIN"),
		password:    os.Getenv("LOOM_SSH_PASSORD"),
		hostname:    os.Getenv("LOOM_SSH_HOSTNAME"),
		port:        os.Getenv("LOOM_SSH_PORT"),
		destination: os.Getenv("LOOM_SSH_DESTINATION"),
	}, nil
}

func (l *Loom) client() (*ssh.Client, error) {
	var client *ssh.Client
	var err error
	port := 22
	if l.port != "" {
		port, err = strconv.Atoi(l.port)
		if err != nil {
			return nil, err
		}
	}
	if l.useAgent {
		client, err = ssh.NewWithAgent(l.hostname, port, l.login, false)
		if err != nil {
			return nil, err
		}
		client.ClientConfig.Timeout = 10 * time.Second
		return client, nil
	}
	client, err = ssh.NewWithPassword(l.hostname, port, l.login, l.password)
	if err != nil {
		return nil, err
	}
	client.ClientConfig.Timeout = 10 * time.Second
	return client, nil
}

func (l *Loom) Remote(command string) error {
	client, err := l.client()
	if err != nil {
		return err
	}
	return client.ExecuteInteractively(command, map[string]string{
		"password:": l.password,
	})
}

func (l *Loom) Upload(src, dst string) error {
	client, err := l.client()
	if err != nil {
		return err
	}
	return client.Upload(src, dst, "0755", false)
}

func (l *Loom) Service(command string) error {
	cmd := fmt.Sprintf("sudo systemctl %s %s", command, l.serviceName)
	return l.Remote(cmd)
}

func (l *Loom) ServiceFile(execStart string) (string, error) {
	ss := systemd.NewService(l.serviceName, execStart)
	filename := fmt.Sprintf("%s.service", l.serviceName)
	err := ss.ToFile(filename)
	if err != nil {
		return "", err
	}
	return filename, err
}

func (l *Loom) UploadToDestination(filename string) error {
	source := fmt.Sprintf("./%s", filename)
	destination := fmt.Sprintf("%s/%s", l.destination, filename)
	return l.Upload(source, destination)
}

func (l *Loom) MoveToOptBin() error {
	cmd := fmt.Sprintf("sudo mv -f %s /opt/%s/bin/%s", l.serviceName, l.serviceName, l.serviceName)
	return l.Remote(cmd)
}

func (l *Loom) Setup(serviceFile string) error {
	err := l.UploadToDestination(serviceFile)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("sudo mv -f %s/%s /etc/systemd/system/%s", l.destination, serviceFile, serviceFile)
	err = l.Remote(cmd)
	if err != nil {
		return err
	}
	cmd = fmt.Sprintf("sudo mkdir -p /opt/%s/bin", l.serviceName)
	err = l.Remote(cmd)
	if err != nil {
		return err
	}
	cmd = fmt.Sprintf("sudo mkdir -p /opt/%s/etc", l.serviceName)
	err = l.Remote(cmd)
	if err != nil {
		return err
	}
	cmd = fmt.Sprintf("sudo mkdir -p /opt/%s/log", l.serviceName)
	err = l.Remote(cmd)
	if err != nil {
		return err
	}
	err = l.UploadToDestination(l.serviceName)
	if err != nil {
		return err
	}
	err = l.MoveToOptBin()
	if err != nil {
		return err
	}
	err = l.Service("enable")
	if err != nil {
		return err
	}
	return nil
}
