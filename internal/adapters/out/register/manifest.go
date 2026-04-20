package register

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Manifest represents the Chrome Native Messaging host manifest.
type Manifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// Config contains the necessary information for registration.
type Config struct {
	Name           string
	Description    string
	BinaryPath     string
	Type           string
	AllowedOrigins []string
}

// NativeHostRegistrar manages the manifest and registration.
type NativeHostRegistrar struct {
	Config   Config
	OS       string
	Home     string
	Registry RegistryHandler
}

// RegistryHandler defines an interface for OS registry operations (Windows specific).
type RegistryHandler interface {
	RegisterHost(name, manifestPath string) error
	UnregisterHost(name string) error
}

// NewRegistrar creates a new NativeHostRegistrar.
func NewRegistrar(cfg Config, os, home string) *NativeHostRegistrar {
	return &NativeHostRegistrar{
		Config: cfg,
		OS:     os,
		Home:   home,
	}
}

// GenerateManifestContent creates the JSON content for the manifest.
func (r *NativeHostRegistrar) GenerateManifestContent() ([]byte, error) {
	if r.Config.Name == "" || r.Config.Description == "" || r.Config.BinaryPath == "" || r.Config.Type == "" {
		return nil, fmt.Errorf("missing mandatory fields in config")
	}

	// Ensure binary path is absolute
	absPath, err := filepath.Abs(r.Config.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for binary: %w", err)
	}

	m := Manifest{
		Name:           r.Config.Name,
		Description:    r.Config.Description,
		Path:           absPath,
		Type:           r.Config.Type,
		AllowedOrigins: r.Config.AllowedOrigins,
	}
	return json.MarshalIndent(m, "", "  ")
}

// GetManifestPath returns the OS-specific path for the manifest file.
func (r *NativeHostRegistrar) GetManifestPath() (string, error) {
	if r.Home == "" {
		return "", fmt.Errorf("home directory is required")
	}

	var path string
	switch r.OS {
	case "windows":
		path = filepath.Join(r.Home, "Google", "Chrome", "NativeMessagingHosts", r.Config.Name+".json")
	case "darwin":
		path = filepath.Join(r.Home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts", r.Config.Name+".json")
	case "linux":
		path = filepath.Join(r.Home, ".config", "google-chrome", "NativeMessagingHosts", r.Config.Name+".json")
	default:
		return "", fmt.Errorf("unsupported platform: %s", r.OS)
	}
	return path, nil
}

// CreateManifest generates the JSON manifest and writes it to the system path.
func (r *NativeHostRegistrar) CreateManifest() (string, error) {
	content, err := r.GenerateManifestContent()
	if err != nil {
		return "", err
	}

	path, err := r.GetManifestPath()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(path)
	// Permissions: 0755 as requested for directories
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	return path, nil
}

// Install performs the full registration (Manifest + Registry on Windows).
func (r *NativeHostRegistrar) Install() error {
	path, err := r.CreateManifest()
	if err != nil {
		return err
	}

	if r.OS == "windows" {
		reg := r.Registry
		if reg == nil {
			reg = NewWindowsRegistry()
		}
		return reg.RegisterHost(r.Config.Name, path)
	}
	return nil
}

// Uninstall removes the manifest and registry key.
func (r *NativeHostRegistrar) Uninstall() error {
	path, err := r.GetManifestPath()
	if err != nil {
		return err
	}

	// Remove manifest file
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove manifest: %w", err)
	}

	if r.OS == "windows" {
		reg := r.Registry
		if reg == nil {
			reg = NewWindowsRegistry()
		}
		return reg.UnregisterHost(r.Config.Name)
	}
	return nil
}
