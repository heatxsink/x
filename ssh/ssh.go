package ssh

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/heatxsink/x/progressbar"
	"github.com/heatxsink/x/term"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Client struct {
	hostname     string
	port         int
	properties   map[string]string
	ClientConfig *ssh.ClientConfig
	client       *ssh.Client
	isConnected  bool
	useAgent     bool
	agentConn    net.Conn
	agentClient  agent.ExtendedAgent
	debug        bool
}

// NewWithAgentContext dials the SSH agent socket under ctx, so the caller
// can bound how long the dial may take.
func NewWithAgentContext(ctx context.Context, hostname string, port int, username string, debug bool) (*Client, error) {
	var d net.Dialer
	sock, err := d.DialContext(ctx, "unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	agentClient := agent.NewClient(sock)
	if debug {
		fmt.Println(agentClient.List())
	}
	signers, err := agentClient.Signers()
	if err != nil {
		_ = sock.Close()
		return nil, err
	}
	client := &Client{
		hostname:   hostname,
		port:       port,
		properties: map[string]string{},
		ClientConfig: &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signers...),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
		useAgent:    true,
		agentConn:   sock,
		agentClient: agentClient,
		debug:       debug,
	}
	return client, nil
}

// NewWithAgent is a context-less wrapper around NewWithAgentContext.
//
// Deprecated: use NewWithAgentContext, which lets the caller bound the
// agent-socket dial.
func NewWithAgent(hostname string, port int, username string, debug bool) (*Client, error) {
	return NewWithAgentContext(context.Background(), hostname, port, username, debug)
}

func NewWithPrivateKey(hostname string, port int, username, privateKeyFilename, privateKeyPassphrase string) (*Client, error) {
	pemBytes, err := os.ReadFile(privateKeyFilename) // #nosec G304 -- privateKeyFilename is caller-controlled
	if err != nil {
		return nil, err
	}
	var signer ssh.Signer
	if privateKeyPassphrase == "" {
		signer, err = ssh.ParsePrivateKey(pemBytes)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(privateKeyPassphrase))
	}
	if err != nil {
		return nil, err
	}
	return &Client{
		hostname:   hostname,
		port:       port,
		properties: map[string]string{},
		ClientConfig: &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
		useAgent: false,
		debug:    false,
	}, nil
}

func NewWithPassword(hostname string, port int, username string, password string) (*Client, error) {
	client := &Client{
		hostname:   hostname,
		port:       port,
		properties: map[string]string{},
		ClientConfig: &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
		useAgent: false,
		debug:    false,
	}
	return client, nil
}

func (c *Client) SetProperty(key, value string) {
	c.properties[key] = value
}

func (c *Client) Connect() error {
	if c.isConnected {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", c.hostname, c.port)
	client, err := ssh.Dial("tcp", addr, c.ClientConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s, %w", c.hostname, err)
	}
	c.client = client
	c.isConnected = true
	if c.useAgent {
		// Best-effort: agent forwarding failure must not break the connection.
		_ = agent.ForwardToAgent(client, c.agentClient)
	}
	return nil
}

func (c *Client) NewSession() (*ssh.Session, error) {
	err := c.Connect()
	if err != nil {
		return nil, err
	}
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session for %s, %w", c.hostname, err)
	}
	if c.useAgent {
		// Best-effort: server may refuse forwarding (e.g. AllowAgentForwarding no).
		_ = agent.RequestAgentForwarding(session)
	}
	return session, nil
}

func (c *Client) Close() error {
	var errs []error
	if c.isConnected && c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close SSH connection: %w", err))
		}
		c.isConnected = false
	}
	if c.agentConn != nil {
		if err := c.agentConn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close agent connection: %w", err))
		}
		c.agentConn = nil
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c *Client) Capture(command string) (string, error) {
	session, err := c.NewSession()
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer session.Close()
	out, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("failed to execute: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) RequestPty(session *ssh.Session) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// request pseudo terminal
	if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
		return fmt.Errorf("pseudo terminal failed: %w", err)
	}
	return nil
}

func (c *Client) Execute(command string) error {
	var wg sync.WaitGroup
	start := term.StartlnWithTime(command)
	session, err := c.NewSession()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()
	stdout, err := session.StdoutPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stderr pipe: %w", err)
	}
	wg.Add(1)
	go term.DisplayLn(stdout, &wg, func(line string) {
		term.Infoln(line)
	})
	wg.Add(1)
	go term.DisplayLn(stderr, &wg, func(line string) {
		term.Warnln(line)
	})
	err = session.Start(command)
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session start: %w", err)
	}
	err = session.Wait()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session wait: %w", err)
	}
	wg.Wait()
	term.EndlnWithTime(time.Since(start), true)
	return nil
}

func (c *Client) ExecuteInteractively(command string, inputMap map[string]string) error {
	// Pre-compile regexes outside the scan loop
	type patternEntry struct {
		regex *regexp.Regexp
		text  string
	}
	patterns := make([]patternEntry, 0, len(inputMap))
	for pattern, text := range inputMap {
		patterns = append(patterns, patternEntry{
			regex: regexp.MustCompile(pattern),
			text:  text,
		})
	}

	var wg sync.WaitGroup
	start := term.StartlnWithTime(command)
	session, err := c.NewSession()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()
	err = c.RequestPty(session)
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("failed to request pty: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stderr pipe: %w", err)
	}
	wg.Add(1)
	go term.DisplayLn(stderr, &wg, func(line string) {
		term.Warnln(line)
	})
	err = session.Start(command)
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("starting the session: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanBytes)
	replacer := strings.NewReplacer("\n", "", "\r", "")
	var line strings.Builder
	for scanner.Scan() {
		b := scanner.Text()
		if b == "\n" {
			term.Infoln(replacer.Replace(line.String()))
			line.Reset()
		}
		line.WriteString(b)
		lineStr := line.String()
		for _, p := range patterns {
			if p.regex.MatchString(lineStr) {
				fmt.Fprintln(stdin, p.text)
			}
		}
	}
	if err = scanner.Err(); err != nil {
		term.Errorln(err)
		return err
	}
	if err = session.Wait(); err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session wait: %w", err)
	}
	term.EndlnWithTime(time.Since(start), true)
	return nil
}

func (c *Client) uploadByReader(r io.Reader, remotePath string, size int64, permission string, debug bool) error {
	session, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()
	w, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	defer w.Close()
	if debug {
		session.Stdout = os.Stdout
	}
	err = session.Start("/usr/bin/scp -qt " + path.Dir(remotePath))
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	go func() {
		pb := progressbar.DefaultBytes(size, "Uploading")
		teeReader := io.TeeReader(r, pb)
		_, _ = fmt.Fprintln(w, "C"+permission, size, path.Base(remotePath))
		if _, err := io.Copy(w, teeReader); err != nil {
			term.Errorln(fmt.Errorf("failed to copy io: %w", err))
		}
		_, _ = fmt.Fprintln(w, "\x00")
		if err = pb.Close(); err != nil {
			term.Errorln(err)
		}
	}()
	err = session.Wait()
	if err != nil {
		if err.Error() == "Process exited with status 1" {
			// Return nil because this is expected successful behavior.
			return nil
		}
		return fmt.Errorf("error on session wait: %w", err)
	}
	return nil
}

func (c *Client) Upload(localPath string, remotePath string, permission string, debug bool) error {
	fh, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer fh.Close()
	stat, err := fh.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat the local file: %w", err)
	}
	r := bufio.NewReader(fh)
	return c.uploadByReader(r, remotePath, stat.Size(), permission, debug)
}
