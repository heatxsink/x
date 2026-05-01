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
	// userMode flips every system-affecting operation to the per-user
	// equivalent: systemctl --user instead of sudo systemctl,
	// ~/.config/systemd/user/ instead of /etc/systemd/system/, and
	// ~/.local/{bin,etc,log} instead of /opt/<service>/{bin,etc,log}.
	// No sudo is invoked in user mode -- the remote login user owns
	// every path touched.
	userMode bool
}

// New returns a Loom configured for system-level service management:
// units installed under /etc/systemd/system/, binary placed under
// /opt/<serviceName>/bin/, and `sudo systemctl <cmd>` for service
// control. The remote user must be able to sudo without a password.
func New(serviceName string, useAgent bool) (*Loom, error) {
	return load(serviceName, useAgent, false)
}

// NewUser returns a Loom configured for per-user service management:
// units under ~/.config/systemd/user/, binary under ~/.local/bin/,
// and `systemctl --user <cmd>` for service control. No sudo. The
// remote login user owns and manages everything.
//
// For lasting `--user` services that survive logout, the operator
// should `loginctl enable-linger` on the remote once -- not loom's
// job, but documented in the project README.
func NewUser(serviceName string, useAgent bool) (*Loom, error) {
	return load(serviceName, useAgent, true)
}

func load(serviceName string, useAgent, userMode bool) (*Loom, error) {
	if err := dotenv.Load(); err != nil {
		return nil, err
	}
	return &Loom{
		serviceName: serviceName,
		useAgent:    useAgent,
		userMode:    userMode,
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
	if l.userMode {
		cmd := fmt.Sprintf("systemctl --user %s %s", command, l.serviceName)
		return l.Remote(ctx, cmd)
	}
	cmd := fmt.Sprintf("sudo systemctl %s %s", command, l.serviceName)
	return l.Remote(ctx, cmd)
}

// systemdUnitDir is the directory the generated unit lands in --
// /etc/systemd/system/ for system mode, ~/.config/systemd/user/ for
// user mode.
func (l *Loom) systemdUnitDir() string {
	if l.userMode {
		return "$HOME/.config/systemd/user"
	}
	return "/etc/systemd/system"
}

// installPrefix is the parent of bin/etc/log on the remote in
// system mode (/opt/<service>). In user mode the layout is XDG:
// the binary goes to $HOME/.local/bin/<service>, config lives at
// $HOME/.config/<service>/ owned by the application, and logs are
// captured by journald via systemd -- no etc/log dirs are created.
// installPrefix is unused in user mode; binPath does the right
// thing per mode.
func (l *Loom) installPrefix() string {
	return fmt.Sprintf("/opt/%s", l.serviceName)
}

// binPath is the final destination of the deployed binary.
//
//	system:  /opt/<service>/bin/<service>
//	user:    $HOME/.local/bin/<service>
func (l *Loom) binPath() string {
	if l.userMode {
		return "$HOME/.local/bin/" + l.serviceName
	}
	return fmt.Sprintf("/opt/%s/bin/%s", l.serviceName, l.serviceName)
}

// sudoPrefix is "sudo " for system mode, empty for user mode --
// every shell-out that touches a privileged path uses this.
func (l *Loom) sudoPrefix() string {
	if l.userMode {
		return ""
	}
	return "sudo "
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

// MoveToOptBin moves the just-uploaded binary into its install
// path. The method name is a holdover from the system-mode-only
// past; it now does the right thing per mode (see binPath).
func (l *Loom) MoveToOptBin(ctx context.Context) error {
	cmd := fmt.Sprintf("%smv -f %s %s",
		l.sudoPrefix(), l.serviceName, l.binPath())
	return l.Remote(ctx, cmd)
}

// Setup installs everything a fresh remote needs:
//
//	system mode:
//	  - uploads serviceFile + binary to staging destination
//	  - sudo mv serviceFile -> /etc/systemd/system/
//	  - sudo mkdir -p /opt/<service>/{bin,etc,log}
//	  - sudo mv binary -> /opt/<service>/bin/
//	  - sudo systemctl enable <service>
//
//	user mode:
//	  - uploads serviceFile + binary to staging destination
//	  - mkdir -p ~/.config/systemd/user ~/.local/bin
//	  - mv serviceFile -> ~/.config/systemd/user/
//	  - mv binary -> ~/.local/bin/<service>
//	  - systemctl --user enable <service>
//	  (no etc/log -- the daemon owns config via XDG paths and logs
//	   land in the journal via systemd's stdout capture)
//
// In user mode no sudo is invoked. In system mode the remote user
// must be able to sudo without a password.
func (l *Loom) Setup(ctx context.Context, serviceFile string) error {
	if err := l.UploadToDestination(ctx, serviceFile); err != nil {
		return err
	}
	if l.userMode {
		mkdirCmd := fmt.Sprintf("mkdir -p %s $HOME/.local/bin", l.systemdUnitDir())
		if err := l.Remote(ctx, mkdirCmd); err != nil {
			return err
		}
	}
	mvCmd := fmt.Sprintf("%smv -f %s/%s %s/%s",
		l.sudoPrefix(), l.destination, serviceFile, l.systemdUnitDir(), serviceFile)
	if err := l.Remote(ctx, mvCmd); err != nil {
		return err
	}
	if !l.userMode {
		mkdirCmd := fmt.Sprintf("sudo mkdir -p %s/bin %s/etc %s/log",
			l.installPrefix(), l.installPrefix(), l.installPrefix())
		if err := l.Remote(ctx, mkdirCmd); err != nil {
			return err
		}
	}
	if err := l.UploadToDestination(ctx, l.serviceName); err != nil {
		return err
	}
	if err := l.MoveToOptBin(ctx); err != nil {
		return err
	}
	return l.Service(ctx, "enable")
}
