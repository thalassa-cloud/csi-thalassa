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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"k8s.io/mount-utils"
	kexec "k8s.io/utils/exec"
)

const (
	runningState = "running"
)

type prodAttachmentValidator struct{}

func (av *prodAttachmentValidator) readFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (av *prodAttachmentValidator) evalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

type AttachmentValidator interface {
	readFile(name string) ([]byte, error)
	evalSymlinks(path string) (string, error)
}

type findmntResponse struct {
	FileSystems []fileSystem `json:"filesystems"`
}

type fileSystem struct {
	Target      string `json:"target"`
	Propagation string `json:"propagation"`
	FsType      string `json:"fstype"`
	Options     string `json:"options"`
}

type volumeStatistics struct {
	availableBytes, totalBytes, usedBytes    int64
	availableInodes, totalInodes, usedInodes int64
}

const (
	// blkidExitStatusNoIdentifiers defines the exit code returned from blkid indicating that no devices have been found. See http://www.polarhome.com/service/man/?qf=blkid&tf=2&of=Alpinelinux for details.
	blkidExitStatusNoIdentifiers = 2
)

// DeviceManager handles device-related operations
type DeviceManager interface {
	// GetDeviceByID returns the device path for a given volume ID
	GetDeviceByID(volumeID string) (string, error)
	// IsDeviceAttached checks if a device is properly attached
	IsDeviceAttached(devicePath string) error
}

// FilesystemManager handles filesystem operations
type FilesystemManager interface {
	// Format formats a device with the specified filesystem
	Format(devicePath, fsType string) error
	// IsFormatted checks if a device is already formatted
	IsFormatted(devicePath string) (bool, error)
	// Resize resizes a filesystem
	// Resize(devicePath, mountPath string) error
}

// MountManager handles mount operations
type MountManager interface {
	// Mount mounts a source to a target
	Mount(source, target, fsType string, options ...string) error
	// Unmount unmounts a target
	Unmount(target string) error
	// IsAttached checks if a device is attached
	IsAttached(source string) error
	// IsMounted checks if a path is mounted
	IsMounted(target string) (bool, error)
	// GetDeviceName gets the device name for a mount path
	GetDeviceName(mounter mount.Interface, mountPath string) (string, error)
}

// StatisticsManager handles volume statistics
type StatisticsManager interface {
	// GetStatistics returns volume statistics
	GetStatistics(volumePath string) (volumeStatistics, error)
	// IsBlockDevice checks if a path is a block device
	IsBlockDevice(volumePath string) (bool, error)
}

// Mounter is responsible for formatting and mounting volumes
type Mounter interface {
	// DeviceManager
	FilesystemManager
	MountManager
	StatisticsManager
}

type mounter struct {
	log                 *slog.Logger
	kMounter            *mount.SafeFormatAndMount
	attachmentValidator AttachmentValidator
}

// NewMounter returns a new mounter instance
func NewMounter(log *slog.Logger) Mounter {
	kMounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      kexec.New(),
	}

	return &mounter{
		kMounter:            kMounter,
		log:                 log,
		attachmentValidator: &prodAttachmentValidator{},
	}
}

func (m *mounter) Format(source, fsType string) error {
	mkfsCmd := fmt.Sprintf("mkfs.%s", fsType)

	_, err := exec.LookPath(mkfsCmd)
	if err != nil {
		if err == exec.ErrNotFound {
			return fmt.Errorf("%q executable not found in $PATH", mkfsCmd)
		}
		return err
	}

	mkfsArgs := []string{}

	if fsType == "" {
		return errors.New("fs type is not specified for formatting the volume")
	}

	if source == "" {
		return errors.New("source is not specified for formatting the volume")
	}

	mkfsArgs = append(mkfsArgs, source)
	if fsType == "ext4" || fsType == "ext3" {
		mkfsArgs = []string{"-F", source}
	}

	m.log.Info("executing format command", "cmd", mkfsCmd, "args", mkfsArgs)
	out, err := exec.Command(mkfsCmd, mkfsArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("formatting disk failed: %v cmd: '%s %s' output: %q",
			err, mkfsCmd, strings.Join(mkfsArgs, " "), string(out))
	}
	return nil
}

func (m *mounter) Mount(source, target, fsType string, opts ...string) error {
	mountCmd := "mount"
	mountArgs := []string{}

	if source == "" {
		return errors.New("source is not specified for mounting the volume")
	}

	if target == "" {
		return errors.New("target is not specified for mounting the volume")
	}

	// This is a raw block device mount. Create the mount point as a file
	// since bind mount device node requires it to be a file
	if fsType == "" {
		// create directory for target, os.Mkdirall is noop if directory exists
		err := os.MkdirAll(filepath.Dir(target), 0750)
		if err != nil {
			return fmt.Errorf("failed to create target directory for raw block bind mount: %v", err)
		}

		file, err := os.OpenFile(target, os.O_CREATE, 0660)
		if err != nil {
			return fmt.Errorf("failed to create target file for raw block bind mount: %v", err)
		}
		file.Close()
	} else {
		mountArgs = append(mountArgs, "-t", fsType)

		// create target, os.Mkdirall is noop if directory exists
		err := os.MkdirAll(target, 0750)
		if err != nil {
			return err
		}
	}

	// By default, xfs does not allow mounting of two volumes with the same filesystem uuid.
	// Force ignore this uuid to be able to mount volume + its clone / restored snapshot on the same node.
	if fsType == "xfs" {
		opts = append(opts, "nouuid")
	}

	if len(opts) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(opts, ","))
	}

	mountArgs = append(mountArgs, source)
	mountArgs = append(mountArgs, target)

	m.log.Info("executing mount command", "cmd", mountCmd, "args", mountArgs)

	out, err := exec.Command(mountCmd, mountArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mounting failed: %v cmd: '%s %s' output: %q",
			err, mountCmd, strings.Join(mountArgs, " "), string(out))
	}

	return nil
}

func (m *mounter) Unmount(target string) error {
	return mount.CleanupMountPoint(target, m.kMounter, true)
}

func (m *mounter) IsAttached(source string) error {
	out, err := m.attachmentValidator.evalSymlinks(source)
	if err != nil {
		return fmt.Errorf("error evaluating the symbolic link %q: %s", source, err)
	}

	_, deviceName := filepath.Split(out)
	if deviceName == "" {
		return fmt.Errorf("error device name is empty for path %s", out)
	}

	deviceStateFilePath := fmt.Sprintf("/sys/class/block/%s/device/state", deviceName)
	deviceStateFileContent, err := m.attachmentValidator.readFile(deviceStateFilePath)
	if err != nil {
		return fmt.Errorf("error reading the device state file %q: %s", deviceStateFilePath, err)
	}

	if string(deviceStateFileContent) != strings.TrimSpace(runningState) {
		return fmt.Errorf("error comparing the state file content, expected: %s, got: %s", runningState, string(deviceStateFileContent))
	}

	return nil
}

func (m *mounter) IsFormatted(source string) (bool, error) {
	if source == "" {
		return false, errors.New("source is not specified")
	}

	blkidCmd := "blkid"
	_, err := exec.LookPath(blkidCmd)
	if err != nil {
		if err == exec.ErrNotFound {
			return false, fmt.Errorf("%q executable not found in $PATH", blkidCmd)
		}
		return false, err
	}

	blkidArgs := []string{source}

	m.log.Info("checking if source is formatted", "cmd", blkidCmd, "args", blkidArgs)

	exitCode := 0
	cmd := exec.Command(blkidCmd, blkidArgs...)
	err = cmd.Run()
	if err != nil {
		exitError, ok := err.(*exec.ExitError)
		if !ok {
			return false, fmt.Errorf("checking formatting failed: %v cmd: %q, args: %q", err, blkidCmd, blkidArgs)
		}
		ws := exitError.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
		if exitCode == blkidExitStatusNoIdentifiers {
			return false, nil
		}
		return false, fmt.Errorf("checking formatting failed: %v cmd: %q, args: %q", err, blkidCmd, blkidArgs)
	}

	return true, nil
}

func (m *mounter) IsMounted(target string) (bool, error) {
	if target == "" {
		return false, errors.New("target is not specified for checking the mount")
	}

	findmntCmd := "findmnt"
	_, err := exec.LookPath(findmntCmd)
	if err != nil {
		if err == exec.ErrNotFound {
			return false, fmt.Errorf("%q executable not found in $PATH", findmntCmd)
		}
		return false, err
	}

	findmntArgs := []string{"-o", "TARGET,PROPAGATION,FSTYPE,OPTIONS", "-M", target, "-J"}

	m.log.Info("checking if target is mounted", "cmd", findmntCmd, "args", findmntArgs)

	out, err := exec.Command(findmntCmd, findmntArgs...).CombinedOutput()
	if err != nil {
		// findmnt exits with non zero exit status if it couldn't find anything
		if strings.TrimSpace(string(out)) == "" {
			return false, nil
		}

		return false, fmt.Errorf("checking mounted failed: %v cmd: %q output: %q",
			err, findmntCmd, string(out))
	}

	// no response means there is no mount
	if string(out) == "" {
		return false, nil
	}

	var resp *findmntResponse
	err = json.Unmarshal(out, &resp)
	if err != nil {
		return false, fmt.Errorf("couldn't unmarshal data: %q: %s", string(out), err)
	}

	targetFound := false
	for _, fs := range resp.FileSystems {
		// check if the mount is propagated correctly. It should be set to shared.
		if fs.Propagation != "shared" {
			return true, fmt.Errorf("mount propagation for target %q is not enabled", target)
		}

		// the mountpoint should match as well
		if fs.Target == target {
			targetFound = true
		}
	}

	return targetFound, nil
}

func (m *mounter) GetDeviceName(mounter mount.Interface, mountPath string) (string, error) {
	devicePath, _, err := mount.GetDeviceNameFromMount(mounter, mountPath)
	return devicePath, err
}

func (m *mounter) GetStatistics(volumePath string) (volumeStatistics, error) {
	isBlock, err := m.IsBlockDevice(volumePath)
	if err != nil {
		return volumeStatistics{}, fmt.Errorf("failed to determine if volume %s is block device: %v", volumePath, err)
	}

	if isBlock {
		// See http://man7.org/linux/man-pages/man8/blockdev.8.html for details
		output, err := exec.Command("blockdev", "getsize64", volumePath).CombinedOutput()
		if err != nil {
			return volumeStatistics{}, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %v", volumePath, string(output), err)
		}
		strOut := strings.TrimSpace(string(output))
		gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
		if err != nil {
			return volumeStatistics{}, fmt.Errorf("failed to parse size %s into int", strOut)
		}

		return volumeStatistics{
			totalBytes: gotSizeBytes,
		}, nil
	}

	var statfs unix.Statfs_t
	// See https://man7.org/linux/man-pages/man2/statfs.2.html for details.
	err = unix.Statfs(volumePath, &statfs)
	if err != nil {
		return volumeStatistics{}, err
	}

	volStats := volumeStatistics{
		availableBytes: int64(statfs.Bavail) * int64(statfs.Bsize),
		totalBytes:     int64(statfs.Blocks) * int64(statfs.Bsize),
		usedBytes:      (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize),

		availableInodes: int64(statfs.Ffree),
		totalInodes:     int64(statfs.Files),
		usedInodes:      int64(statfs.Files) - int64(statfs.Ffree),
	}

	return volStats, nil
}

func (m *mounter) IsBlockDevice(devicePath string) (bool, error) {
	var stat unix.Stat_t
	err := unix.Stat(devicePath, &stat)
	if err != nil {
		return false, err
	}
	return (stat.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}
