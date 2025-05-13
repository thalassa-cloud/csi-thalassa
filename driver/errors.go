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
	"fmt"
)

// Error types for the node driver
var (
	ErrDeviceNotFound     = fmt.Errorf("device not found")
	ErrDeviceNotAttached  = fmt.Errorf("device not attached")
	ErrDeviceNotFormatted = fmt.Errorf("device not formatted")
	ErrMountFailed        = fmt.Errorf("mount failed")
	ErrUnmountFailed      = fmt.Errorf("unmount failed")
	ErrInvalidPath        = fmt.Errorf("invalid path")
	ErrInvalidVolumeID    = fmt.Errorf("invalid volume ID")
	ErrInvalidMountPoint  = fmt.Errorf("invalid mount point")
)

// DeviceError represents an error related to device operations
type DeviceError struct {
	Op  string
	Err error
}

func (e *DeviceError) Error() string {
	return fmt.Sprintf("device error: %s: %v", e.Op, e.Err)
}

func (e *DeviceError) Unwrap() error {
	return e.Err
}

// MountError represents an error related to mount operations
type MountError struct {
	Op  string
	Err error
}

func (e *MountError) Error() string {
	return fmt.Sprintf("mount error: %s: %v", e.Op, e.Err)
}

func (e *MountError) Unwrap() error {
	return e.Err
}

// FilesystemError represents an error related to filesystem operations
type FilesystemError struct {
	Op  string
	Err error
}

func (e *FilesystemError) Error() string {
	return fmt.Sprintf("filesystem error: %s: %v", e.Op, e.Err)
}

func (e *FilesystemError) Unwrap() error {
	return e.Err
}
