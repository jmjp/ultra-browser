//go:build !windows

package register

import "errors"

// DummyRegistry is a placeholder for non-Windows platforms.
type DummyRegistry struct{}

// NewWindowsRegistry returns a dummy registry for non-Windows platforms.
func NewWindowsRegistry() RegistryHandler {
	return &DummyRegistry{}
}

// RegisterHost returns an error on non-Windows platforms.
func (d *DummyRegistry) RegisterHost(name, manifestPath string) error {
	return errors.New("registry not supported on this platform")
}

// UnregisterHost returns an error on non-Windows platforms.
func (d *DummyRegistry) UnregisterHost(name string) error {
	return errors.New("registry not supported on this platform")
}
