package loom

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/heatxsink/x/dotenv"
	"github.com/heatxsink/x/ssh"
	"github.com/heatxsink/x/systemd"
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
	err := dotenv.Load()
	if err != nil {
		return nil, err
	}
	return &Loom{
		serviceName: serviceName,
		useAgent:    useAgent,
		login:       os.Getenv("LOOM_SSH_LOGIN"),
		password:    os.Getenv("LOOM_SSH_PASSWORD"),
		hostname:    os.Getenv("LOOM_SSH_HOSTNAME"),
		port:        os.Getenv("LOOM_SSH_PORT"),
		destination: os.Getenv("LOOM_SSH_DESTINATION"),
	}, nil
}

func (l *Loom) client(ctx context.Context) (*ssh.Client, error) {
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
		client, err = ssh.NewWithAgentContext(ctx, l.hostname, port, l.login, false)
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

func (l *Loom) Remote(ctx context.Context, command string) error {
	client, err := l.client(ctx)
	if err != nil {
		return err
	}
	return client.ExecuteInteractively(command, map[string]string{
		"password:": l.password,
	})
}

func (l *Loom) Upload(ctx context.Context, src, dst string) error {
	client, err := l.client(ctx)
	if err != nil {
		return err
	}
	return client.Upload(src, dst, "0755", false)
}

func (l *Loom) Service(ctx context.Context, command string) error {
	cmd := fmt.Sprintf("sudo systemctl %s %s", command, l.serviceName)
	return l.Remote(ctx, cmd)
}

func (l *Loom) ServiceFile(execStart string) (string, error) {
	ss := systemd.NewService(l.serviceName, execStart)
	filename := fmt.Sprintf("%s.service", l.serviceName)
	if err := ss.ToFile(filename); err != nil {
		return "", err
	}
	return filename, nil
}

func (l *Loom) UploadToDestination(ctx context.Context, filename string) error {
	source := fmt.Sprintf("./%s", filename)
	destination := fmt.Sprintf("%s/%s", l.destination, filename)
	return l.Upload(ctx, source, destination)
}

func (l *Loom) MoveToOptBin(ctx context.Context) error {
	cmd := fmt.Sprintf("sudo mv -f %s /opt/%s/bin/%s", l.serviceName, l.serviceName, l.serviceName)
	return l.Remote(ctx, cmd)
}

func (l *Loom) Setup(ctx context.Context, serviceFile string) error {
	if err := l.UploadToDestination(ctx, serviceFile); err != nil {
		return err
	}
	cmd := fmt.Sprintf("sudo mv -f %s/%s /etc/systemd/system/%s", l.destination, serviceFile, serviceFile)
	if err := l.Remote(ctx, cmd); err != nil {
		return err
	}
	cmd = fmt.Sprintf("sudo mkdir -p /opt/%s/{bin,etc,log}", l.serviceName)
	if err := l.Remote(ctx, cmd); err != nil {
		return err
	}
	if err := l.UploadToDestination(ctx, l.serviceName); err != nil {
		return err
	}
	if err := l.MoveToOptBin(ctx); err != nil {
		return err
	}
	return l.Service(ctx, "enable")
}
