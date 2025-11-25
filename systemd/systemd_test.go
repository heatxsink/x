package systemd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	name := "test-service"
	execStart := "/usr/bin/test-command"

	service := NewService(name, execStart)

	if service.Name != name {
		t.Errorf("Expected Name to be '%s', got '%s'", name, service.Name)
	}
	if service.ExecStart != execStart {
		t.Errorf("Expected ExecStart to be '%s', got '%s'", execStart, service.ExecStart)
	}
	if service.User != "root" {
		t.Errorf("Expected default User to be 'root', got '%s'", service.User)
	}
	if service.Restart != "always" {
		t.Errorf("Expected default Restart to be 'always', got '%s'", service.Restart)
	}
	if service.RestartSec != 3 {
		t.Errorf("Expected default RestartSec to be 3, got %d", service.RestartSec)
	}
	if service.TimeoutStartSec != 0 {
		t.Errorf("Expected default TimeoutStartSec to be 0, got %d", service.TimeoutStartSec)
	}
	if service.After != "" {
		t.Errorf("Expected default After to be empty, got '%s'", service.After)
	}
	if service.Requires != "" {
		t.Errorf("Expected default Requires to be empty, got '%s'", service.Requires)
	}
}

func TestServiceCustomization(t *testing.T) {
	service := NewService("custom-service", "/bin/custom")

	// Customize the service
	service.User = "nobody"
	service.After = "network.target"
	service.Requires = "network-online.target"
	service.TimeoutStartSec = 30
	service.Restart = "on-failure"
	service.RestartSec = 5

	if service.User != "nobody" {
		t.Errorf("Expected User to be 'nobody', got '%s'", service.User)
	}
	if service.After != "network.target" {
		t.Errorf("Expected After to be 'network.target', got '%s'", service.After)
	}
	if service.Requires != "network-online.target" {
		t.Errorf("Expected Requires to be 'network-online.target', got '%s'", service.Requires)
	}
	if service.TimeoutStartSec != 30 {
		t.Errorf("Expected TimeoutStartSec to be 30, got %d", service.TimeoutStartSec)
	}
	if service.Restart != "on-failure" {
		t.Errorf("Expected Restart to be 'on-failure', got '%s'", service.Restart)
	}
	if service.RestartSec != 5 {
		t.Errorf("Expected RestartSec to be 5, got %d", service.RestartSec)
	}
}

func TestServiceToFileDefaults(t *testing.T) {
	service := NewService("test-service", "/usr/bin/test-app")

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-service-*.service")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Generate service file
	err = service.ToFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ToFile failed: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	contentStr := string(content)

	// Check required sections
	if !strings.Contains(contentStr, "[Unit]") {
		t.Error("Service file should contain [Unit] section")
	}
	if !strings.Contains(contentStr, "[Service]") {
		t.Error("Service file should contain [Service] section")
	}
	if !strings.Contains(contentStr, "[Install]") {
		t.Error("Service file should contain [Install] section")
	}

	// Check specific values
	if !strings.Contains(contentStr, "Description=test-service") {
		t.Error("Service file should contain correct Description")
	}
	if !strings.Contains(contentStr, "ExecStart=/usr/bin/test-app") {
		t.Error("Service file should contain correct ExecStart")
	}
	if !strings.Contains(contentStr, "User=root") {
		t.Error("Service file should contain correct user")
	}
	if !strings.Contains(contentStr, "Restart=always") {
		t.Error("Service file should contain correct Restart")
	}
	if !strings.Contains(contentStr, "RestartSec=3") {
		t.Error("Service file should contain correct RestartSec")
	}
	if !strings.Contains(contentStr, "TimeoutStartSec=0") {
		t.Error("Service file should contain correct TimeoutStartSec")
	}
	if !strings.Contains(contentStr, "WantedBy=multi-user.target") {
		t.Error("Service file should contain correct WantedBy")
	}
}

func TestServiceToFileCustomValues(t *testing.T) {
	service := NewService("custom-service", "/opt/app/bin/myapp --config=/etc/myapp.conf")
	service.User = "appuser"
	service.After = "network.target postgresql.service"
	service.Requires = "postgresql.service"
	service.TimeoutStartSec = 60
	service.Restart = "on-failure"
	service.RestartSec = 10

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "custom-service-*.service")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Generate service file
	err = service.ToFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ToFile failed: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	contentStr := string(content)

	// Check custom values
	if !strings.Contains(contentStr, "Description=custom-service") {
		t.Error("Service file should contain correct Description")
	}
	if !strings.Contains(contentStr, "After=network.target postgresql.service") {
		t.Error("Service file should contain correct After")
	}
	if !strings.Contains(contentStr, "Requires=postgresql.service") {
		t.Error("Service file should contain correct Requires")
	}
	if !strings.Contains(contentStr, "User=appuser") {
		t.Error("Service file should contain correct user")
	}
	if !strings.Contains(contentStr, "ExecStart=/opt/app/bin/myapp --config=/etc/myapp.conf") {
		t.Error("Service file should contain correct ExecStart with arguments")
	}
	if !strings.Contains(contentStr, "TimeoutStartSec=60") {
		t.Error("Service file should contain correct TimeoutStartSec")
	}
	if !strings.Contains(contentStr, "Restart=on-failure") {
		t.Error("Service file should contain correct Restart")
	}
	if !strings.Contains(contentStr, "RestartSec=10") {
		t.Error("Service file should contain correct RestartSec")
	}
}

func TestServiceToFileInvalidPath(t *testing.T) {
	service := NewService("test-service", "/usr/bin/test")

	// Try to write to an invalid path
	err := service.ToFile("/invalid/path/that/does/not/exist/test.service")
	if err == nil {
		t.Error("ToFile should fail when writing to invalid path")
	}
}

func TestServiceToFileEmptyFields(t *testing.T) {
	service := NewService("", "")

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "empty-service-*.service")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Generate service file
	err = service.ToFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ToFile failed: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	contentStr := string(content)

	// Check that empty values are handled
	if !strings.Contains(contentStr, "Description=") {
		t.Error("Service file should contain Description field even if empty")
	}
	if !strings.Contains(contentStr, "ExecStart=") {
		t.Error("Service file should contain ExecStart field even if empty")
	}
}

func TestServiceToFilePermissions(t *testing.T) {
	service := NewService("perm-test", "/bin/echo")

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "systemd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filename := filepath.Join(tmpDir, "test.service")

	// Generate service file
	err = service.ToFile(filename)
	if err != nil {
		t.Fatalf("ToFile failed: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Service file was not created")
	}

	// Check file content is readable
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read service file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Service file should not be empty")
	}
}

func TestServiceToFileMultipleServices(t *testing.T) {
	services := []*Service{
		NewService("service1", "/bin/app1"),
		NewService("service2", "/bin/app2"),
		NewService("service3", "/bin/app3"),
	}

	// Customize each service
	services[0].User = "user1"
	services[1].User = "user2"
	services[2].User = "user3"

	tmpDir, err := os.MkdirTemp("", "systemd-multi-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate multiple service files
	for i, service := range services {
		filename := filepath.Join(tmpDir, service.Name+".service")
		err := service.ToFile(filename)
		if err != nil {
			t.Errorf("ToFile failed for service %d: %v", i, err)
			continue
		}

		// Verify each file
		content, err := os.ReadFile(filename)
		if err != nil {
			t.Errorf("Failed to read service file %d: %v", i, err)
			continue
		}

		contentStr := string(content)
		expectedUser := services[i].User
		if !strings.Contains(contentStr, "User="+expectedUser) {
			t.Errorf("Service file %d should contain User=%s", i, expectedUser)
		}
	}
}
