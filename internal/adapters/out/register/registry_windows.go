//go:build windows

package register

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// WindowsRegistry implements RegistryHandler for the Windows platform.
type WindowsRegistry struct{}

// NewWindowsRegistry returns a new WindowsRegistry.
func NewWindowsRegistry() RegistryHandler {
	return &WindowsRegistry{}
}

// RegisterHost creates the registry key for Chrome Native Messaging.
func (w *WindowsRegistry) RegisterHost(name, manifestPath string) error {
	keyPath := `Software\Google\Chrome\NativeMessagingHosts\` + name
	k, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to create registry key: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("", manifestPath); err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	return nil
}

// UnregisterHost deletes the registry key.
func (w *WindowsRegistry) UnregisterHost(name string) error {
	keyPath := `Software\Google\Chrome\NativeMessagingHosts\` + name
	err := registry.DeleteKey(registry.CURRENT_USER, keyPath)
	if err != nil {
		// If key doesn't exist, it's fine for unregistering
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to delete registry key: %w", err)
	}
	return nil
}
