package sshkit

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

	"github.com/heatxsink/x/termkit"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Config struct {
	Hostname             string `json:"hostname,omitempty"`
	Port                 int    `json:"port,omitempty"`
	Username             string `json:"username,omitempty"`
	Password             string `json:"password,omitempty"`
	PrivateKeyFilename   string `json:"private_key_filename,omitempty"`
	PrivateKeyPassphrase string `json:"private_key_passphrase,omitempty"`
	UseAgent             bool   `json:"use_agent,omitempty"`
}

type Client struct {
	config       *Config
	properties   map[string]string
	ClientConfig *ssh.ClientConfig
	client       *ssh.Client
	isConnected  bool
	useAgent     bool
	agentClient  agent.ExtendedAgent
}

func NewWithAgent(config *Config, debug bool) (*Client, error) {
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
		return nil, fmt.Errorf("signers error: %v", err)
	}
	client := &Client{
		config:     config,
		properties: map[string]string{},
		ClientConfig: &ssh.ClientConfig{
			User: config.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signers...),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
		useAgent:    true,
		agentClient: agentClient,
	}
	return client, nil
}

func New(config *Config) (*Client, error) {
	if config.Password == "" && config.PrivateKeyFilename == "" {
		return nil, fmt.Errorf("failed to construct ssh client both password and private key are empty")
	}
	var authMethod ssh.AuthMethod
	var signer ssh.Signer
	var err error
	if config.PrivateKeyFilename != "" && config.PrivateKeyPassphrase != "" {
		signer, err = signerFromKeyFileAndPassphrase(config.PrivateKeyFilename, config.PrivateKeyPassphrase)
		if err != nil {
			return nil, err
		}
		authMethod = ssh.PublicKeys(signer)
	} else if config.PrivateKeyFilename != "" {
		var err error
		signer, err = signerFromKeyFile(config.PrivateKeyFilename)
		if err != nil {
			return nil, fmt.Errorf("failed to get public keys from supplied keyfile, %v", err)
		}
		authMethod = ssh.PublicKeys(signer)
	} else if config.Password != "" {
		authMethod = ssh.Password(config.Password)
	}
	client := &Client{
		config:     config,
		properties: map[string]string{},
		ClientConfig: &ssh.ClientConfig{
			User: config.Username,
			Auth: []ssh.AuthMethod{
				authMethod,
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
		useAgent: false,
	}
	return client, nil
}

func signerFromKeyFile(keyfile string) (ssh.Signer, error) {
	pemBytes, err := os.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(pemBytes)
}

func signerFromKeyFileAndPassphrase(keyFile string, passphrase string) (ssh.Signer, error) {
	pemBytes, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passphrase))
}

func (c *Client) SetProperty(key, value string) {
	c.properties[key] = value
}

func (c *Client) Connect() error {
	if c.isConnected {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", c.config.Hostname, c.config.Port)
	client, err := ssh.Dial("tcp", addr, c.ClientConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s, %v", c.config.Hostname, err)
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
		return nil, fmt.Errorf("failed to create SSH session for %s, %v", c.config.Hostname, err)
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
	start := termkit.StartlnWithTime(command)
	session, err := c.NewSession()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("create session: %v", err)
	}
	defer session.Close()
	stdout, err := session.StdoutPipe()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdout pipe: %v", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stderr pipe: %v", err)
	}
	wg.Add(1)
	go termkit.DisplayLn(stdout, &wg, func(line string) {
		termkit.Infoln(line)
	})
	wg.Add(1)
	go termkit.DisplayLn(stderr, &wg, func(line string) {
		termkit.Warnln(line)
	})
	err = session.Start(command)
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session start: %v", err)
	}
	err = session.Wait()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session wait: %v", err)
	}
	wg.Wait()
	termkit.EndlnWithTime(time.Since(start), true)
	return nil
}

func (c *Client) ExecuteInteractively(command string, inputMap map[string]string) error {
	var wg sync.WaitGroup
	start := termkit.StartlnWithTime(command)
	session, err := c.NewSession()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("create session: %v", err)
	}
	defer session.Close()
	err = c.RequestPty(session)
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("failed to request pty: %v", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdin pipe: %v", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stdout pipe: %v", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("stderr pipe: %v", err)
	}
	wg.Add(1)
	go termkit.DisplayLn(stderr, &wg, func(line string) {
		termkit.Warnln(line)
	})
	err = session.Start(command)
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
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
			termkit.Infoln(d)
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
		termkit.Errorln(err)
		return err
	}
	err = session.Wait()
	if err != nil {
		termkit.EndlnWithTime(time.Since(start), false)
		return fmt.Errorf("session wait: %v", err)
	}
	termkit.EndlnWithTime(time.Since(start), true)
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
		p := NewProgressWriter(size, "Uploading", "Uploaded")
		teeReader := io.TeeReader(r, p)
		fmt.Fprintln(w, "C"+permission, size, path.Base(remotePath))
		_, err := io.Copy(w, teeReader)
		if err != nil {
			termkit.Errorln(fmt.Errorf("failed to copy io: %v", err))
		}
		fmt.Fprintln(w, "\x00")
		p.Stop()
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
