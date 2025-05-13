/*
Copyright 2025 Thalassa Cloud

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"k8s.io/mount-utils"
)

// MockMounter implements the Mounter interface for testing
type MockMounter struct {
	// MountPoints tracks mounted paths
	MountPoints map[string]string
	// FormattedDevices tracks formatted devices
	FormattedDevices map[string]string
	// AttachedDevices tracks attached devices
	AttachedDevices map[string]bool
	// Statistics tracks volume statistics
	Statistics map[string]volumeStatistics
	// BlockDevices tracks block devices
	BlockDevices map[string]bool
	// MountErrors allows injecting errors for mount operations
	MountErrors map[string]error
	// UnmountErrors allows injecting errors for unmount operations
	UnmountErrors map[string]error
	// FormatErrors allows injecting errors for format operations
	FormatErrors map[string]error
}

// NewMockMounter creates a new MockMounter
func NewMockMounter() *MockMounter {
	return &MockMounter{
		MountPoints:      make(map[string]string),
		FormattedDevices: make(map[string]string),
		AttachedDevices:  make(map[string]bool),
		Statistics:       make(map[string]volumeStatistics),
		BlockDevices:     make(map[string]bool),
		MountErrors:      make(map[string]error),
		UnmountErrors:    make(map[string]error),
		FormatErrors:     make(map[string]error),
	}
}

// GetDeviceByID implements DeviceManager
func (m *MockMounter) GetDeviceByID(volumeID string) (string, error) {
	if device, ok := m.MountPoints[volumeID]; ok {
		return device, nil
	}
	return "", ErrDeviceNotFound
}

// IsDeviceAttached implements DeviceManager
func (m *MockMounter) IsDeviceAttached(devicePath string) error {
	if !m.AttachedDevices[devicePath] {
		return ErrDeviceNotAttached
	}
	return nil
}

// Format implements FilesystemManager
func (m *MockMounter) Format(devicePath, fsType string) error {
	if err := m.FormatErrors[devicePath]; err != nil {
		return err
	}
	m.FormattedDevices[devicePath] = fsType
	return nil
}

// IsAttached implements FilesystemManager
func (m *MockMounter) IsAttached(source string) error {
	if !m.AttachedDevices[source] {
		return ErrDeviceNotAttached
	}
	return nil
}

// IsFormatted implements FilesystemManager
func (m *MockMounter) IsFormatted(devicePath string) (bool, error) {
	_, ok := m.FormattedDevices[devicePath]
	return ok, nil
}

// Resize implements FilesystemManager
func (m *MockMounter) Resize(devicePath, mountPath string) error {
	return nil
}

// Mount implements MountManager
func (m *MockMounter) Mount(source, target, fsType string, options ...string) error {
	if err := m.MountErrors[target]; err != nil {
		return err
	}
	m.MountPoints[target] = source
	return nil
}

// Unmount implements MountManager
func (m *MockMounter) Unmount(target string) error {
	if err := m.UnmountErrors[target]; err != nil {
		return err
	}
	delete(m.MountPoints, target)
	return nil
}

// IsMounted implements MountManager
func (m *MockMounter) IsMounted(target string) (bool, error) {
	_, ok := m.MountPoints[target]
	return ok, nil
}

// GetDeviceName implements MountManager
func (m *MockMounter) GetDeviceName(mounter mount.Interface, mountPath string) (string, error) {
	if device, ok := m.MountPoints[mountPath]; ok {
		return device, nil
	}
	return "", ErrDeviceNotFound
}

// GetStatistics implements StatisticsManager
func (m *MockMounter) GetStatistics(volumePath string) (volumeStatistics, error) {
	if stats, ok := m.Statistics[volumePath]; ok {
		return stats, nil
	}
	return volumeStatistics{}, nil
}

// IsBlockDevice implements StatisticsManager
func (m *MockMounter) IsBlockDevice(volumePath string) (bool, error) {
	return m.BlockDevices[volumePath], nil
}
