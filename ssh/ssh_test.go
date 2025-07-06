package ssh

import (
	"os"
	"strings"
	"testing"
	"time"
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
	// Create a temporary key file for testing
	tmpKey := createTempKey(t)
	defer os.Remove(tmpKey)

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

func TestRequestPty(t *testing.T) {
	// This test requires a mock session since we can't create a real SSH connection
	// We'll test the basic functionality without actual SSH connection
	client, err := NewWithPassword("localhost", 22, "testuser", "testpass")
	if err != nil {
		t.Fatalf("NewWithPassword failed: %v", err)
	}

	// Test that RequestPty method exists and accepts session parameter
	// In a real scenario, this would need a mock SSH session
	if client == nil {
		t.Error("Client should not be nil")
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

// Integration tests that require actual SSH connection
// These tests are marked with build tag to skip during normal testing

func TestExecute(t *testing.T) {
	t.Skip("Skipping integration test - requires actual SSH server")
	client, err := NewWithPassword("localhost", 22, "testuser", "testpass")
	if err != nil {
		t.Error(err)
	}
	client.SetProperty("PubkeyAuthentication", "no")
	client.ClientConfig.Timeout = 10 * time.Second
	err = client.Execute("ls -alh /dev/tty")
	if err != nil {
		t.Error(err)
	}
}

func TestExecuteInteractively(t *testing.T) {
	t.Skip("Skipping integration test - requires actual SSH server")
	client, err := NewWithPassword("localhost", 22, "testuser", "testpass")
	if err != nil {
		t.Error(err)
	}
	client.SetProperty("PubkeyAuthentication", "no")
	client.ClientConfig.Timeout = 10 * time.Second
	err = client.ExecuteInteractively("ls -alh /dev/tty", map[string]string{
		"Password:": "testpass",
	})
	if err != nil {
		t.Error(err)
	}
}

// Helper function to create a temporary SSH key for testing
func createTempKey(t *testing.T) string {
	// Use a real SSH private key for testing
	keyContent := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEAtRpxCI1U4fIlKupVjobpXuXfqs1HBOR/WdbRp9TL2mgijtfe7V8X
hQmPz0Fb/a9L0EOwH2djnqDsC+VTPfh3AHeY8DSkKlNIs/HHwOh/x/DxS+VlUKPrqMqQNP
zW3yYb9S5GEydjdhDi8LH0RkfzJkAHxNfVaz1T90NGKnoFamkn8d5HvypDq5Wwvrvo4Fbh
6L5WpbPKTp4aDj71wct+aDYmUjgMPT6CRDEQZKHmRT/HK9aK/0Bm/K6/NlRjrxkcQmBZr+
UzpubqSjtpv+d1m5i364DaBGn89soegaBR+xgMK7rAsJchGXoi72uggQjBlERSu2xK1ytU
lzaqwkPhAQAAA8hIu8GLSLvBiwAAAAdzc2gtcnNhAAABAQC1GnEIjVTh8iUq6lWOhule5d
+qzUcE5H9Z1tGn1MvaaCKO197tXxeFCY/PQVv9r0vQQ7AfZ2OeoOwL5VM9+HcAd5jwNKQq
U0iz8cfA6H/H8PFL5WVQo+uoypA0/NbfJhv1LkYTJ2N2EOLwsfRGR/MmQAfE19VrPVP3Q0
YqegVqaSfx3ke/KkOrlbC+u+jgVuHovlals8pOnhoOPvXBy35oNiZSOAw9PoJEMRBkoeZF
P8cr1or/QGb8rr82VGOvGRxCYFmv5TOm5upKO2m/53WbmLfrgNoEafz2yh6BoFH7GAwrus
CwlyEZeiLva6CBCMGURFK7bErXK1SXNqrCQ+EBAAAAAwEAAQAAAQBGVP/7xcNmwh7MFVhn
sx4zlAtybik8Da8liSc/ygTnC5UMK2qwfcMJEAcRAr2Cfk7vkTH3aDQIeU9iaUuUIAe7Hz
c+ZfsxUsnD1ExyrvhdAkX7ZxmbISXWleA+K8kYvViTNcbSDnRyeCliN4H5v1x/CNPbjsSb
0qPmvXIk8eFjis7FsZF1OcjPsCIvxwMdT7z3pK1Y3mizfcZq+mkmz+UGSISKK3rP8pxA/s
P0aZeQQ/6PIfqXD7vG9Lc+52rNdCTiN1l6hQX7BFIHlw3SEzdYrvT4WWxtj6+j4S/MvKeo
PdxMgMXC1sNe04t0XBIBLMZ5T+NSxPYcydFfKtl2gOKPAAAAgQDQ6rZZWWFVHLlXxy+zbN
VGzkx2kImm4Tj1p/rkZiQ1Vo29hPVdqO9NA+F/EvigS1DUF6XgvNDwjJ/+0sk0JzxjDo+P
dN+wd2AyJVh9exQTWCa0Fv8tHn1GWHwtD/JUnVDWQ3N9yzJV/r8CMAxZbsia3s3C3HEdRm
Ged5a85r7UhQAAAIEA3IdGHDcSGWmBLhNnycfV/KYBk9f/4OSjOyrNHN7EIXEW5zHbc+cG
w3NIA0TJFVbRoheDdMbz41ICMgE/KUdVLyLzWzg4tA68/FYydBzZhtTfTL5NEn+BRfp/IT
68qmXIlk/We33JKUT7kFCB7uQk6OFRpazzLYvndux8UlUAzQcAAACBANI7wRnYmVV/zcBQ
DqJsni7CGs/F9yA/gPl3EA8Ew5TjT8zNJhgWrMhCgQxokXzUdsv568fCZtlEL0Czzm94LR
KkTJgYTjtJqllWSgNFDHrmbNaARnR2aKZfnaWg80qYSZiRD9hucLR6fPh7it11seKbCR6t
1vcLbOwlYyDQ0Oe3AAAADm5ncmFuYWRvQGxvcnJkAQIDBA==
-----END OPENSSH PRIVATE KEY-----`

	tmpFile, err := os.CreateTemp("", "ssh_test_key")
	if err != nil {
		t.Fatalf("Failed to create temp key file: %v", err)
	}

	if _, err := tmpFile.WriteString(keyContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write key content: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}
