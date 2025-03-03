package ssh

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/heatxsink/x/term"
	"github.com/schollz/progressbar/v3"
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
	agentClient  agent.ExtendedAgent
	debug        bool
}

func NewWithAgent(hostname string, port int, username string, debug bool) (*Client, error) {
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	agentClient := agent.NewClient(sock)
	if debug {
		fmt.Println(agentClient.List())
	}
	signers, err := agentClient.Signers()
	if err != nil {
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
		agentClient: agentClient,
		debug:       debug,
	}
	return client, nil
}

func NewWithPrivateKey(hostname string, port int, username string, privateKeyFilename string, privateKeyPassphrase string) (*Client, error) {
	var signer ssh.Signer
	if privateKeyPassphrase == "" {
		pemBytes, err := os.ReadFile(privateKeyFilename)
		if err != nil {
			return nil, err
		}
		signer, err = ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, err
		}
	} else {
		pemBytes, err := os.ReadFile(privateKeyFilename)
		if err != nil {
			return nil, err
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(privateKeyPassphrase))
		if err != nil {
			return nil, err
		}
	}
	client := &Client{
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
	}
	return client, nil
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
		return fmt.Errorf("failed to connect to %s, %v", c.hostname, err)
	}
	c.client = client
	c.isConnected = true
	if c.useAgent {
		agent.ForwardToAgent(client, c.agentClient)
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
		return nil, fmt.Errorf("failed to create SSH session for %s, %v", c.hostname, err)
	}
	if c.useAgent {
		agent.RequestAgentForwarding(session)
	}
	return session, nil
}

func (c *Client) Close() error {
	if !c.isConnected {
		return nil
	}
	err := c.client.Close()
	if err != nil {
		return fmt.Errorf("failed to close SSH connection %v", err)
	}
	return nil
}

func (c *Client) Capture(command string) (string, error) {
	session, err := c.NewSession()
	if err != nil {
		return "", fmt.Errorf("create session: %v", err)
	}
	defer session.Close()
	out, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("failed to execute: %v", err)
	}
	result := strings.TrimSpace(string(out[:]))
	return result, nil
}

func (c *Client) RequestPty(session *ssh.Session) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	//request pseudo terminal
	if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
		return fmt.Errorf("pseudo terminal failed: %v", err)
	}
	return nil
}

func (c *Client) Execute(command string) error {
	var wg sync.WaitGroup
	start := term.StartlnWithTime(command)
	session, err := c.NewSession()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("create session: %v", err)
	}
	defer session.Close()
	stdout, err := session.StdoutPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdout pipe: %v", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stderr pipe: %v", err)
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
		return fmt.Errorf("session start: %v", err)
	}
	err = session.Wait()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session wait: %v", err)
	}
	wg.Wait()
	term.EndlnWithTime(time.Since(start), true)
	return nil
}

func (c *Client) ExecuteInteractively(command string, inputMap map[string]string) error {
	var wg sync.WaitGroup
	start := term.StartlnWithTime(command)
	session, err := c.NewSession()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("create session: %v", err)
	}
	defer session.Close()
	err = c.RequestPty(session)
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("failed to request pty: %v", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdin pipe: %v", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdout pipe: %v", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stderr pipe: %v", err)
	}
	wg.Add(1)
	go term.DisplayLn(stderr, &wg, func(line string) {
		term.Warnln(line)
	})
	err = session.Start(command)
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("starting the session: %v", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanBytes)
	var line strings.Builder
	for scanner.Scan() {
		b := scanner.Text()
		if b == "\n" {
			d := strings.Replace(line.String(), "\n", "", -1)
			d = strings.Replace(d, "\r", "", -1)
			term.Infoln(d)
			line.Reset()
		}
		line.WriteString(b)
		for pattern, text := range inputMap {
			reg := regexp.MustCompile(pattern)
			if reg.MatchString(line.String()) {
				fmt.Fprintln(stdin, text)
			}
		}
	}
	err = scanner.Err()
	if err != nil {
		term.Errorln(err)
		return err
	}
	err = session.Wait()
	if err != nil {
		term.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session wait: %v", err)
	}
	term.EndlnWithTime(time.Since(start), true)
	return nil
}

func (c *Client) uploadByReader(r io.Reader, remotePath string, size int64, permission string, debug bool) error {
	session, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()
	w, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}
	defer w.Close()
	if debug {
		session.Stdout = os.Stdout
	}
	err = session.Start("/usr/bin/scp -qt " + path.Dir(remotePath))
	if err != nil {
		return fmt.Errorf("failed to start session: %v", err)
	}
	go func() {
		pb := progressbar.DefaultBytes(-1, "Uploading")
		teeReader := io.TeeReader(r, pb)
		fmt.Fprintln(w, "C"+permission, size, path.Base(remotePath))
		_, err := io.Copy(w, teeReader)
		if err != nil {
			term.Errorln(fmt.Errorf("failed to copy io: %v", err))
		}
		fmt.Fprintln(w, "\x00")
	}()
	err = session.Wait()
	if err != nil {
		if err.Error() == "Process exited with status 1" {
			// Return nil because this is expected successful behavior.
			return nil
		}
		return fmt.Errorf("error on session wait: %v", err)
	}
	return nil
}

func (c *Client) Upload(localPath string, remotePath string, permission string, debug bool) error {
	fh, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer fh.Close()
	stat, err := fh.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat the local file: %v", err)
	}
	r := bufio.NewReader(fh)
	return c.uploadByReader(r, remotePath, stat.Size(), permission, debug)
}
