package loom

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		useAgent    bool
		envVars     map[string]string
		wantErr     bool
	}{
		{
			name:        "valid service with agent",
			serviceName: "test-service",
			useAgent:    true,
			envVars: map[string]string{
				"LOOM_SSH_LOGIN":       "testuser",
				"LOOM_SSH_PASSORD":     "testpass",
				"LOOM_SSH_HOSTNAME":    "testhost",
				"LOOM_SSH_PORT":        "2222",
				"LOOM_SSH_DESTINATION": "/tmp/test",
			},
			wantErr: false,
		},
		{
			name:        "valid service without agent",
			serviceName: "test-service-2",
			useAgent:    false,
			envVars: map[string]string{
				"LOOM_SSH_LOGIN":       "testuser2",
				"LOOM_SSH_PASSORD":     "testpass2",
				"LOOM_SSH_HOSTNAME":    "testhost2",
				"LOOM_SSH_PORT":        "22",
				"LOOM_SSH_DESTINATION": "/tmp/test2",
			},
			wantErr: false,
		},
		{
			name:        "minimal config",
			serviceName: "minimal-service",
			useAgent:    true,
			envVars: map[string]string{
				"LOOM_SSH_LOGIN":    "user",
				"LOOM_SSH_HOSTNAME": "host",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			loom, err := New(tt.serviceName, tt.useAgent)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if loom.serviceName != tt.serviceName {
					t.Errorf("New() serviceName = %v, want %v", loom.serviceName, tt.serviceName)
				}
				if loom.useAgent != tt.useAgent {
					t.Errorf("New() useAgent = %v, want %v", loom.useAgent, tt.useAgent)
				}
				if loom.login != tt.envVars["LOOM_SSH_LOGIN"] {
					t.Errorf("New() login = %v, want %v", loom.login, tt.envVars["LOOM_SSH_LOGIN"])
				}
				if loom.password != tt.envVars["LOOM_SSH_PASSORD"] {
					t.Errorf("New() password = %v, want %v", loom.password, tt.envVars["LOOM_SSH_PASSORD"])
				}
				if loom.hostname != tt.envVars["LOOM_SSH_HOSTNAME"] {
					t.Errorf("New() hostname = %v, want %v", loom.hostname, tt.envVars["LOOM_SSH_HOSTNAME"])
				}
				if loom.port != tt.envVars["LOOM_SSH_PORT"] {
					t.Errorf("New() port = %v, want %v", loom.port, tt.envVars["LOOM_SSH_PORT"])
				}
				if loom.destination != tt.envVars["LOOM_SSH_DESTINATION"] {
					t.Errorf("New() destination = %v, want %v", loom.destination, tt.envVars["LOOM_SSH_DESTINATION"])
				}
			}
		})
	}
}

func TestLoom_client(t *testing.T) {
	tests := []struct {
		name        string
		loom        *Loom
		expectError bool
	}{
		{
			name: "invalid port",
			loom: &Loom{
				hostname: "testhost",
				port:     "invalid",
				login:    "testuser",
				password: "testpass",
				useAgent: false,
			},
			expectError: true,
		},
		{
			name: "empty port defaults to 22",
			loom: &Loom{
				hostname: "testhost",
				port:     "",
				login:    "testuser",
				password: "testpass",
				useAgent: false,
			},
			expectError: true,
		},
		{
			name: "valid port string",
			loom: &Loom{
				hostname: "testhost",
				port:     "2222",
				login:    "testuser",
				password: "testpass",
				useAgent: false,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.loom.client()
			if (err != nil) != tt.expectError {
				t.Errorf("client() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestLoom_ServiceFile(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		execStart   string
		wantErr     bool
	}{
		{
			name:        "valid service file",
			serviceName: "test-service",
			execStart:   "/opt/test-service/bin/test-service",
			wantErr:     false,
		},
		{
			name:        "empty exec start",
			serviceName: "test-service",
			execStart:   "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loom := &Loom{
				serviceName: tt.serviceName,
			}
			filename, err := loom.ServiceFile(tt.execStart)
			if (err != nil) != tt.wantErr {
				t.Errorf("ServiceFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				expectedFilename := tt.serviceName + ".service"
				if filename != expectedFilename {
					t.Errorf("ServiceFile() filename = %v, want %v", filename, expectedFilename)
				}
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					t.Errorf("ServiceFile() did not create file %v", filename)
				} else {
					os.Remove(filename)
				}
			}
		})
	}
}

func TestLoom_UploadToDestination(t *testing.T) {
	tests := []struct {
		name        string
		loom        *Loom
		filename    string
		expectError bool
	}{
		{
			name: "valid destination",
			loom: &Loom{
				destination: "/tmp/test",
			},
			filename:    "test.txt",
			expectError: true,
		},
		{
			name: "empty destination",
			loom: &Loom{
				destination: "",
			},
			filename:    "test.txt",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.loom.UploadToDestination(tt.filename)
			if (err != nil) != tt.expectError {
				t.Errorf("UploadToDestination() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestLoom_Remote(t *testing.T) {
	loom := &Loom{
		hostname: "nonexistent-host",
		port:     "22",
		login:    "testuser",
		password: "testpass",
		useAgent: false,
	}

	err := loom.Remote("echo 'test'")
	if err == nil {
		t.Error("Remote() expected error for nonexistent host, got nil")
	}
}

func TestLoom_Upload(t *testing.T) {
	loom := &Loom{
		hostname: "nonexistent-host",
		port:     "22",
		login:    "testuser",
		password: "testpass",
		useAgent: false,
	}

	err := loom.Upload("test.txt", "/tmp/test.txt")
	if err == nil {
		t.Error("Upload() expected error for nonexistent host, got nil")
	}
}

func TestLoom_Service(t *testing.T) {
	loom := &Loom{
		hostname:    "nonexistent-host",
		port:        "22",
		login:       "testuser",
		password:    "testpass",
		useAgent:    false,
		serviceName: "test-service",
	}

	err := loom.Service("start")
	if err == nil {
		t.Error("Service() expected error for nonexistent host, got nil")
	}
}

func TestLoom_MoveToOptBin(t *testing.T) {
	loom := &Loom{
		hostname:    "nonexistent-host",
		port:        "22",
		login:       "testuser",
		password:    "testpass",
		useAgent:    false,
		serviceName: "test-service",
	}

	err := loom.MoveToOptBin()
	if err == nil {
		t.Error("MoveToOptBin() expected error for nonexistent host, got nil")
	}
}

func TestLoom_Setup(t *testing.T) {
	loom := &Loom{
		hostname:    "nonexistent-host",
		port:        "22",
		login:       "testuser",
		password:    "testpass",
		useAgent:    false,
		serviceName: "test-service",
		destination: "/tmp/test",
	}

	err := loom.Setup("test-service.service")
	if err == nil {
		t.Error("Setup() expected error for nonexistent host, got nil")
	}
}
