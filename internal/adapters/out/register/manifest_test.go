package register

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// mockRegistry implements RegistryHandler for testing purposes.
type mockRegistry struct {
	calledRegister   bool
	calledUnregister bool
	name             string
	path             string
}

func (m *mockRegistry) RegisterHost(name, manifestPath string) error {
	m.calledRegister = true
	m.name = name
	m.path = manifestPath
	return nil
}

func (m *mockRegistry) UnregisterHost(name string) error {
	m.calledUnregister = true
	m.name = name
	return nil
}

func TestGenerateManifestContent(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: Config{
				Name:           "com.ultra_browser.host",
				Description:    "Ultra Browser Host",
				BinaryPath:     "ultra-browser",
				Type:           "stdio",
				AllowedOrigins: []string{"chrome-extension://abcdefg/"},
			},
			wantErr: false,
		},
		{
			name: "Missing Name",
			config: Config{
				Description: "No name",
				BinaryPath:  "sh",
				Type:        "stdio",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistrar(tt.config, "linux", "/tmp")
			data, err := r.GenerateManifestContent()

			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateManifestContent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				var m Manifest
				if err := json.Unmarshal(data, &m); err != nil {
					t.Fatalf("Failed to unmarshal JSON: %v", err)
				}

				if m.Name != tt.config.Name {
					t.Errorf("Expected Name %s, got %s", tt.config.Name, m.Name)
				}
				if m.Description != tt.config.Description {
					t.Errorf("Expected Description %s, got %s", tt.config.Description, m.Description)
				}
				// BinaryPath will be converted to absolute, so we check if it contains our input
				if !filepath.IsAbs(m.Path) {
					t.Errorf("Expected absolute path, got %s", m.Path)
				}
				if m.Type != tt.config.Type {
					t.Errorf("Expected Type %s, got %s", tt.config.Type, m.Type)
				}
				if len(m.AllowedOrigins) != len(tt.config.AllowedOrigins) || m.AllowedOrigins[0] != tt.config.AllowedOrigins[0] {
					t.Errorf("AllowedOrigins mismatch")
				}
			}
		})
	}
}

func TestGetManifestPath(t *testing.T) {
	name := "com.ultra_browser.host"
	cfg := Config{Name: name}

	tests := []struct {
		os       string
		home     string
		expected string
	}{
		{
			os:       "windows",
			home:     `C:\Users\João\AppData\Roaming`,
			expected: filepath.Join(`C:\Users\João\AppData\Roaming`, "Google", "Chrome", "NativeMessagingHosts", name+".json"),
		},
		{
			os:       "darwin",
			home:     "/Users/joao",
			expected: filepath.Join("/Users/joao", "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts", name+".json"),
		},
		{
			os:       "linux",
			home:     "/home/joao",
			expected: filepath.Join("/home/joao", ".config", "google-chrome", "NativeMessagingHosts", name+".json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			r := NewRegistrar(cfg, tt.os, tt.home)
			path, err := r.GetManifestPath()
			if err != nil {
				t.Fatalf("GetManifestPath() error = %v", err)
			}
			if path != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, path)
			}
		})
	}
}

func TestCreateManifest(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{
		Name:           "com.ultra_browser.host",
		Description:    "Test Host",
		BinaryPath:     "echo",
		Type:           "stdio",
		AllowedOrigins: []string{"ext"},
	}

	// Mocking linux to test path creation
	r := NewRegistrar(cfg, "linux", tempDir)
	path, err := r.CreateManifest()
	if err != nil {
		t.Fatalf("CreateManifest() error = %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Manifest file was not created at %s", path)
	}

	// Check directory permissions (on Unix-like systems)
	if runtime.GOOS != "windows" {
		dir := filepath.Dir(path)
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("Expected directory permissions 0755, got %v", info.Mode().Perm())
		}
	}

	// Verify content
	data, _ := os.ReadFile(path)
	var m Manifest
	json.Unmarshal(data, &m)
	if m.Name != cfg.Name {
		t.Errorf("Content mismatch: expected %s, got %s", cfg.Name, m.Name)
	}
}

func TestInstall(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{
		Name:           "com.ultra_browser.host",
		Description:    "Test Host",
		BinaryPath:     "echo",
		Type:           "stdio",
		AllowedOrigins: []string{"ext"},
	}

	mock := &mockRegistry{}
	r := NewRegistrar(cfg, "windows", tempDir)
	r.Registry = mock

	err := r.Install()
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if !mock.calledRegister {
		t.Error("Registry RegisterHost was not called")
	}

	path, _ := r.GetManifestPath()
	if mock.path != path {
		t.Errorf("Registry called with wrong path: expected %s, got %s", path, mock.path)
	}
}

func TestUninstall(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{
		Name: "com.ultra_browser.host",
	}

	mock := &mockRegistry{}
	r := NewRegistrar(cfg, "windows", tempDir)
	r.Registry = mock

	// Create a dummy file to be removed
	path, _ := r.GetManifestPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("{}"), 0644)

	err := r.Uninstall()
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	if !mock.calledUnregister {
		t.Error("Registry UnregisterHost was not called")
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Manifest file still exists after Uninstall")
	}
}
