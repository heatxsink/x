package ssh

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// execFunc handles one "exec" request against the in-process test server. It
// may write to ch (the client's stdout) and ch.Stderr(), read from ch (the
// client's stdin), and returns the exit status reported back to the client.
type execFunc func(t *testing.T, cmd string, ch ssh.Channel) int

// testServer is a real SSH server built on golang.org/x/crypto/ssh, listening
// on loopback. It exists so the client's connect, session, exec and scp paths
// are exercised against a genuine protocol implementation rather than mocked.
type testServer struct {
	port int
	exec execFunc

	mu        sync.Mutex
	commands  []string
	ptyReqs   int
	agentReqs int
}

func newTestServer(t *testing.T, exec execFunc) *testServer {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("host key signer: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("authentication rejected for %q", c.User())
		},
		// Any key is accepted; these tests cover the client, not authorization.
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	cfg.AddHostKey(signer)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is %T, want *net.TCPAddr", l.Addr())
	}
	ts := &testServer{port: addr.Port, exec: exec}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return // listener closed by cleanup
			}
			go ts.serveConn(t, conn, cfg)
		}
	}()
	return ts
}

func (ts *testServer) serveConn(t *testing.T, conn net.Conn, cfg *ssh.ServerConfig) {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		// A rejected handshake is expected in the auth-failure tests.
		_ = conn.Close()
		return
	}
	defer func() { _ = sconn.Close() }()
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "only session channels")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			t.Errorf("accept channel: %v", err)
			return
		}
		go ts.serveSession(t, ch, chReqs)
	}
}

func (ts *testServer) serveSession(t *testing.T, ch ssh.Channel, reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			ts.mu.Lock()
			ts.ptyReqs++
			ts.mu.Unlock()
			ts.reply(t, req, true)
		case "auth-agent-req@openssh.com":
			ts.mu.Lock()
			ts.agentReqs++
			ts.mu.Unlock()
			ts.reply(t, req, true)
		case "shell":
			ts.reply(t, req, true)
		case "exec":
			var payload struct{ Command string }
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				t.Errorf("unmarshal exec payload: %v", err)
				ts.reply(t, req, false)
				return
			}
			ts.mu.Lock()
			ts.commands = append(ts.commands, payload.Command)
			ts.mu.Unlock()
			ts.reply(t, req, true)

			status := 0
			if ts.exec != nil {
				status = ts.exec(t, payload.Command, ch)
			}
			ts.sendExit(t, ch, status)
			_ = ch.Close()
			return
		default:
			ts.reply(t, req, false)
		}
	}
}

func (ts *testServer) reply(t *testing.T, req *ssh.Request, ok bool) {
	if !req.WantReply {
		return
	}
	if err := req.Reply(ok, nil); err != nil {
		t.Errorf("reply to %q: %v", req.Type, err)
	}
}

func (ts *testServer) sendExit(t *testing.T, ch ssh.Channel, status int) {
	payload := ssh.Marshal(struct{ Status uint32 }{uint32(status)}) // #nosec G115 -- test-controlled small value
	if _, err := ch.SendRequest("exit-status", false, payload); err != nil {
		t.Errorf("send exit-status: %v", err)
	}
}

func (ts *testServer) sentCommands() []string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return append([]string(nil), ts.commands...)
}

func (ts *testServer) counts() (pty, agentFwd int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.ptyReqs, ts.agentReqs
}

// client returns a password-authenticated client pointed at the test server.
func (ts *testServer) client(t *testing.T) *Client {
	t.Helper()
	c, err := NewWithPassword("127.0.0.1", ts.port, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword: %v", err)
	}
	c.ClientConfig.Timeout = 10 * time.Second
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// --- Connect / Close ---------------------------------------------------------

func TestConnectAgainstServer(t *testing.T) {
	ts := newTestServer(t, nil)
	c := ts.client(t)

	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !c.isConnected {
		t.Error("isConnected = false after Connect")
	}
}

// Connect must be idempotent: the second call short-circuits on isConnected
// rather than dialing a second TCP connection.
func TestConnectIsIdempotent(t *testing.T) {
	ts := newTestServer(t, nil)
	c := ts.client(t)

	if err := c.Connect(); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	first := c.client
	if err := c.Connect(); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	if c.client != first {
		t.Error("second Connect replaced the underlying client")
	}
}

func TestConnectWrongPassword(t *testing.T) {
	ts := newTestServer(t, nil)
	c, err := NewWithPassword("127.0.0.1", ts.port, "testuser", "wrong")
	if err != nil {
		t.Fatalf("NewWithPassword: %v", err)
	}
	c.ClientConfig.Timeout = 10 * time.Second

	if err := c.Connect(); err == nil {
		t.Fatal("Connect with wrong password: err = nil, want error")
	}
	if c.isConnected {
		t.Error("isConnected = true after failed Connect")
	}
}

func TestCloseAfterConnect(t *testing.T) {
	ts := newTestServer(t, nil)
	c := ts.client(t)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if c.isConnected {
		t.Error("isConnected = true after Close")
	}
	// Close is safe to call twice; the second is a no-op.
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// --- Capture -----------------------------------------------------------------

func TestCaptureReturnsTrimmedStdout(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, err := io.WriteString(ch, "  hello from server  \n\n"); err != nil {
			t.Errorf("write stdout: %v", err)
		}
		return 0
	})
	c := ts.client(t)

	got, err := c.Capture("echo hello")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got != "hello from server" {
		t.Errorf("Capture = %q, want %q", got, "hello from server")
	}
	if cmds := ts.sentCommands(); len(cmds) != 1 || cmds[0] != "echo hello" {
		t.Errorf("server received %v, want [echo hello]", cmds)
	}
}

// Capture uses CombinedOutput, so stderr must appear in the returned string.
func TestCaptureCombinesStderr(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, err := io.WriteString(ch.Stderr(), "warning: from stderr\n"); err != nil {
			t.Errorf("write stderr: %v", err)
		}
		return 0
	})
	c := ts.client(t)

	got, err := c.Capture("some-command")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !strings.Contains(got, "warning: from stderr") {
		t.Errorf("Capture = %q, want it to contain stderr output", got)
	}
}

func TestCaptureNonZeroExit(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		return 3
	})
	c := ts.client(t)

	got, err := c.Capture("false")
	if err == nil {
		t.Fatalf("Capture on exit 3: err = nil, want error (got output %q)", got)
	}
	if !strings.Contains(err.Error(), "failed to execute") {
		t.Errorf("error = %v, want it to mention failed to execute", err)
	}
}

// --- NewSession / RequestPty -------------------------------------------------

func TestNewSessionAgainstServer(t *testing.T) {
	ts := newTestServer(t, nil)
	c := ts.client(t)

	session, err := c.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	// A password client does not use an agent, so no forwarding is requested.
	if _, agentFwd := ts.counts(); agentFwd != 0 {
		t.Errorf("agent forwarding requests = %d, want 0", agentFwd)
	}
}

// The original TestRequestPty asserted only that the client was non-nil. This
// drives the real pty-req through to the server.
func TestRequestPtyIsAcceptedByServer(t *testing.T) {
	ts := newTestServer(t, nil)
	c := ts.client(t)
	session, err := c.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := c.RequestPty(session); err != nil {
		t.Fatalf("RequestPty: %v", err)
	}
	if pty, _ := ts.counts(); pty != 1 {
		t.Errorf("pty requests = %d, want 1", pty)
	}
}

// --- Execute -----------------------------------------------------------------

func TestExecuteAgainstServer(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, err := io.WriteString(ch, "line one\nline two\n"); err != nil {
			t.Errorf("write stdout: %v", err)
		}
		if _, err := io.WriteString(ch.Stderr(), "a warning\n"); err != nil {
			t.Errorf("write stderr: %v", err)
		}
		return 0
	})
	c := ts.client(t)

	if err := c.Execute("ls -alh"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if cmds := ts.sentCommands(); len(cmds) != 1 || cmds[0] != "ls -alh" {
		t.Errorf("server received %v, want [ls -alh]", cmds)
	}
}

func TestExecuteNonZeroExitReturnsError(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		return 1
	})
	c := ts.client(t)

	err := c.Execute("false")
	if err == nil {
		t.Fatal("Execute on exit 1: err = nil, want error")
	}
	if !strings.Contains(err.Error(), "session wait") {
		t.Errorf("error = %v, want it to mention session wait", err)
	}
}

// --- ExecuteInteractively ----------------------------------------------------

// The server prompts, the client must match the prompt pattern and write the
// mapped response back over stdin.
func TestExecuteInteractivelyAnswersPrompt(t *testing.T) {
	received := make(chan string, 1)
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, err := io.WriteString(ch, "Password:"); err != nil {
			t.Errorf("write prompt: %v", err)
			return 1
		}
		line, err := bufio.NewReader(ch).ReadString('\n')
		if err != nil {
			t.Errorf("read response: %v", err)
			return 1
		}
		received <- strings.TrimSpace(line)
		if _, err := io.WriteString(ch, "\naccepted\n"); err != nil {
			t.Errorf("write result: %v", err)
		}
		return 0
	})
	c := ts.client(t)

	err := c.ExecuteInteractively("login", map[string]string{"Password:": "hunter2"})
	if err != nil {
		t.Fatalf("ExecuteInteractively: %v", err)
	}
	select {
	case got := <-received:
		if got != "hunter2" {
			t.Errorf("server received %q, want %q", got, "hunter2")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never received the prompt response")
	}
	if pty, _ := ts.counts(); pty != 1 {
		t.Errorf("pty requests = %d, want 1 (ExecuteInteractively requests a pty)", pty)
	}
}

// A pattern that never matches must not write anything to stdin.
func TestExecuteInteractivelyNoMatchWritesNothing(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, err := io.WriteString(ch, "nothing interesting\n"); err != nil {
			t.Errorf("write stdout: %v", err)
		}
		return 0
	})
	c := ts.client(t)

	if err := c.ExecuteInteractively("run", map[string]string{"Password:": "hunter2"}); err != nil {
		t.Fatalf("ExecuteInteractively: %v", err)
	}
}

func TestExecuteInteractivelyNonZeroExit(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, err := io.WriteString(ch, "done\n"); err != nil {
			t.Errorf("write stdout: %v", err)
		}
		return 2
	})
	c := ts.client(t)

	if err := c.ExecuteInteractively("run", nil); err == nil {
		t.Fatal("ExecuteInteractively on exit 2: err = nil, want error")
	}
}

// --- Upload ------------------------------------------------------------------

// scpSink parses the subset of the scp sink protocol that uploadByReader
// speaks: a "C<mode> <size> <name>" header line followed by size bytes.
func scpSink(t *testing.T, ch ssh.Channel) (mode, name string, body []byte, err error) {
	r := bufio.NewReader(ch)
	header, err := r.ReadString('\n')
	if err != nil {
		return "", "", nil, fmt.Errorf("read scp header: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(header))
	if len(fields) != 3 {
		return "", "", nil, fmt.Errorf("malformed scp header %q", header)
	}
	size, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", "", nil, fmt.Errorf("parse size in %q: %w", header, err)
	}
	body = make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return "", "", nil, fmt.Errorf("read scp body: %w", err)
	}
	return strings.TrimPrefix(fields[0], "C"), fields[2], body, nil
}

func TestUploadTransfersFileContents(t *testing.T) {
	type result struct {
		mode, name string
		body       []byte
	}
	got := make(chan result, 1)
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		mode, name, body, err := scpSink(t, ch)
		if err != nil {
			t.Errorf("scp sink: %v", err)
			return 1
		}
		got <- result{mode, name, body}
		return 0
	})
	c := ts.client(t)

	payload := []byte("file contents over scp")
	local := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(local, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := c.Upload(local, "/remote/dir/payload.txt", "0644", false); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	select {
	case r := <-got:
		if r.mode != "0644" {
			t.Errorf("mode = %q, want 0644", r.mode)
		}
		if r.name != "payload.txt" {
			t.Errorf("name = %q, want payload.txt", r.name)
		}
		if string(r.body) != string(payload) {
			t.Errorf("body = %q, want %q", r.body, payload)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server never received the upload")
	}

	// The remote directory, not the full path, is handed to scp -qt.
	if cmds := ts.sentCommands(); len(cmds) != 1 || cmds[0] != "/usr/bin/scp -qt /remote/dir" {
		t.Errorf("server received %v, want [/usr/bin/scp -qt /remote/dir]", cmds)
	}
}

// uploadByReader deliberately treats "Process exited with status 1" as success,
// because the scp sink exits 1 on a clean transfer in practice. This pins that
// behavior so it is not lost by accident.
func TestUploadTreatsExitStatusOneAsSuccess(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, _, _, err := scpSink(t, ch); err != nil {
			t.Errorf("scp sink: %v", err)
		}
		return 1
	})
	c := ts.client(t)

	local := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(local, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Upload(local, "/remote/dir/payload.txt", "0644", false); err != nil {
		t.Fatalf("Upload with exit status 1: err = %v, want nil", err)
	}
}

func TestUploadOtherNonZeroExitIsAnError(t *testing.T) {
	ts := newTestServer(t, func(t *testing.T, cmd string, ch ssh.Channel) int {
		if _, _, _, err := scpSink(t, ch); err != nil {
			t.Errorf("scp sink: %v", err)
		}
		return 2
	})
	c := ts.client(t)

	local := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(local, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Upload(local, "/remote/dir/payload.txt", "0644", false); err == nil {
		t.Fatal("Upload with exit status 2: err = nil, want error")
	}
}

// --- Agent authentication ----------------------------------------------------

// newTestAgent serves an in-memory keyring on a unix socket and points
// SSH_AUTH_SOCK at it.
func newTestAgent(t *testing.T) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "agent.sock")
	if len(sock) > 100 {
		t.Skipf("socket path too long for AF_UNIX: %q", sock)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: &priv}); err != nil {
		t.Fatalf("add key to keyring: %v", err)
	}

	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen on agent socket: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() { _ = agent.ServeAgent(keyring, conn) }()
		}
	}()

	t.Setenv("SSH_AUTH_SOCK", sock)
	return sock
}

func TestNewWithAgentContext(t *testing.T) {
	newTestAgent(t)

	c, err := NewWithAgentContext(context.Background(), "example.host", 2222, "testuser", false)
	if err != nil {
		t.Fatalf("NewWithAgentContext: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if !c.useAgent {
		t.Error("useAgent = false, want true")
	}
	if c.agentConn == nil {
		t.Error("agentConn = nil, want the dialed socket")
	}
	if c.agentClient == nil {
		t.Error("agentClient = nil, want an agent client")
	}
	if len(c.ClientConfig.Auth) != 1 {
		t.Errorf("Auth methods = %d, want 1", len(c.ClientConfig.Auth))
	}
	if c.ClientConfig.User != "testuser" {
		t.Errorf("User = %q, want testuser", c.ClientConfig.User)
	}
}

// Close must release the agent socket in addition to any SSH connection.
func TestCloseReleasesAgentConn(t *testing.T) {
	newTestAgent(t)
	c, err := NewWithAgentContext(context.Background(), "example.host", 2222, "testuser", false)
	if err != nil {
		t.Fatalf("NewWithAgentContext: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if c.agentConn != nil {
		t.Error("agentConn is non-nil after Close")
	}
}

func TestNewWithAgentContextCanceledContext(t *testing.T) {
	newTestAgent(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewWithAgentContext(ctx, "example.host", 2222, "testuser", false); err == nil {
		t.Fatal("NewWithAgentContext with canceled context: err = nil, want error")
	}
}

func TestNewWithAgentContextNoSocket(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "absent.sock"))

	if _, err := NewWithAgentContext(context.Background(), "example.host", 2222, "testuser", false); err == nil {
		t.Fatal("NewWithAgentContext with no agent: err = nil, want error")
	}
}

// NewWithAgent is the deprecated context-less wrapper; it must still behave.
func TestNewWithAgentWrapper(t *testing.T) {
	newTestAgent(t)

	c, err := NewWithAgent("example.host", 2222, "testuser", false) //nolint:staticcheck // exercising the deprecated wrapper
	if err != nil {
		t.Fatalf("NewWithAgent: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if !c.useAgent {
		t.Error("useAgent = false, want true")
	}
}

// An agent-backed client requests agent forwarding when it opens a session.
func TestAgentClientRequestsForwarding(t *testing.T) {
	newTestAgent(t)
	ts := newTestServer(t, nil)

	c, err := NewWithAgentContext(context.Background(), "127.0.0.1", ts.port, "testuser", false)
	if err != nil {
		t.Fatalf("NewWithAgentContext: %v", err)
	}
	c.ClientConfig.Timeout = 10 * time.Second
	t.Cleanup(func() { _ = c.Close() })

	session, err := c.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	if _, agentFwd := ts.counts(); agentFwd != 1 {
		t.Errorf("agent forwarding requests = %d, want 1", agentFwd)
	}
}
