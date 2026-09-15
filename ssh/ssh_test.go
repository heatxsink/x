package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestNewWithPassword(t *testing.T) {
	client, err := NewWithPassword("localhost", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	if client.hostname != "localhost" {
		t.Errorf("Expected hostname 'localhost', got '%s'", client.hostname)
	}
	if client.port != 22 {
		t.Errorf("Expected port 22, got %d", client.port)
	}
	if client.ClientConfig.User != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", client.ClientConfig.User)
	}
	if client.useAgent {
		t.Error("Expected useAgent to be false for password auth")
	}
	if client.isConnected {
		t.Error("Expected isConnected to be false initially")
	}
}

func TestNewWithPrivateKey(t *testing.T) {
	tmpKey := writeTestKey(t)

	client, err := NewWithPrivateKey("localhost", 22, "testuser", tmpKey, "")
	if err != nil {
		t.Fatalf("NewWithPrivateKey failed: %v", err)
	}

	if client.hostname != "localhost" {
		t.Errorf("Expected hostname 'localhost', got '%s'", client.hostname)
	}
	if client.useAgent {
		t.Error("Expected useAgent to be false for private key auth")
	}
}

func TestNewWithPrivateKeyInvalidFile(t *testing.T) {
	_, err := NewWithPrivateKey("localhost", 22, "testuser", "/nonexistent/key", "")
	if err == nil {
		t.Error("Expected error for nonexistent private key file")
	}
}

func TestSetProperty(t *testing.T) {
	client, err := NewWithPassword("localhost", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	client.SetProperty("TestKey", "TestValue")
	if client.properties["TestKey"] != "TestValue" {
		t.Errorf("Expected property 'TestKey' to be 'TestValue', got '%s'", client.properties["TestKey"])
	}
}

func TestConnectWithoutServer(t *testing.T) {
	client, err := NewWithPassword("nonexistent.host", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	client.ClientConfig.Timeout = 1 * time.Second
	err = client.Connect()
	if err == nil {
		t.Error("Expected error when connecting to nonexistent host")
	}
}

func TestNewSessionWithoutConnection(t *testing.T) {
	client, err := NewWithPassword("nonexistent.host", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	client.ClientConfig.Timeout = 1 * time.Second
	_, err = client.NewSession()
	if err == nil {
		t.Error("Expected error when creating session without connection")
	}
}

func TestCloseWithoutConnection(t *testing.T) {
	client, err := NewWithPassword("localhost", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close should not fail when not connected: %v", err)
	}
}

func TestCaptureWithoutConnection(t *testing.T) {
	client, err := NewWithPassword("nonexistent.host", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	client.ClientConfig.Timeout = 1 * time.Second
	_, err = client.Capture("echo test")
	if err == nil {
		t.Error("Expected error when capturing output without connection")
	}
}

func TestUploadByReaderErrors(t *testing.T) {
	client, err := NewWithPassword("nonexistent.host", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	client.ClientConfig.Timeout = 1 * time.Second
	reader := strings.NewReader("test content")
	err = client.uploadByReader(reader, "/tmp/test", 12, "0644", false)
	if err == nil {
		t.Error("Expected error when uploading without connection")
	}
}

func TestUploadWithInvalidFile(t *testing.T) {
	client, err := NewWithPassword("localhost", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	err = client.Upload("/nonexistent/file", "/tmp/test", "0644", false)
	if err == nil {
		t.Error("Expected error when uploading nonexistent file")
	}
}

// testKeyPassphrase protects the shared encrypted test key.
const testKeyPassphrase = "correct horse battery staple"

// encryptedTestKey marshals one passphrase-protected key for the whole package
// run. The bcrypt KDF behind MarshalPrivateKeyWithPassphrase costs over a
// second per call, so marshalling one per test tripled this package's runtime.
var encryptedTestKey = sync.OnceValues(func() ([]byte, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "x-ssh-tests", []byte(testKeyPassphrase))
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(block), nil
})

// writeKeyFile writes PEM bytes to a per-test file and returns its path.
func writeKeyFile(t *testing.T, pemBytes []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	return path
}

// writeTestKey generates an unencrypted private key per call and writes it in
// OpenSSH format. Generating keys per run keeps real key material out of the
// repository; an embedded key here was published in every release for as long
// as it existed.
func writeTestKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "x-ssh-tests")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return writeKeyFile(t, pem.EncodeToMemory(block))
}

// writeEncryptedTestKey writes the shared passphrase-protected key.
func writeEncryptedTestKey(t *testing.T) string {
	t.Helper()
	pemBytes, err := encryptedTestKey()
	if err != nil {
		t.Fatalf("marshal encrypted private key: %v", err)
	}
	return writeKeyFile(t, pemBytes)
}

// A passphrase-protected key takes the ParsePrivateKeyWithPassphrase branch,
// which no test previously reached.
func TestNewWithPrivateKeyPassphrase(t *testing.T) {
	tmpKey := writeEncryptedTestKey(t)

	client, err := NewWithPrivateKey("localhost", 22, "testuser", tmpKey, testKeyPassphrase)
	if err != nil {
		t.Fatalf("NewWithPrivateKey with passphrase: %v", err)
	}
	if len(client.ClientConfig.Auth) != 1 {
		t.Errorf("Auth methods = %d, want 1", len(client.ClientConfig.Auth))
	}
}

func TestNewWithPrivateKeyWrongPassphrase(t *testing.T) {
	tmpKey := writeEncryptedTestKey(t)

	if _, err := NewWithPrivateKey("localhost", 22, "testuser", tmpKey, "the wrong one"); err == nil {
		t.Fatal("NewWithPrivateKey with the wrong passphrase: err = nil, want error")
	}
}

// An encrypted key with no passphrase must fail rather than be parsed.
func TestNewWithPrivateKeyMissingPassphrase(t *testing.T) {
	tmpKey := writeEncryptedTestKey(t)

	if _, err := NewWithPrivateKey("localhost", 22, "testuser", tmpKey, ""); err == nil {
		t.Fatal("NewWithPrivateKey without the required passphrase: err = nil, want error")
	}
}

func TestNewWithPrivateKeyMalformed(t *testing.T) {
	path := writeKeyFile(t, []byte("not a key"))

	if _, err := NewWithPrivateKey("localhost", 22, "testuser", path, ""); err == nil {
		t.Fatal("NewWithPrivateKey on a malformed key: err = nil, want error")
	}
}
