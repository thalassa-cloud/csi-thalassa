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
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNodeStageVolume(t *testing.T) {
	tests := []struct {
		name          string
		req           *csi.NodeStageVolumeRequest
		mockSetup     func(*MockMounter)
		expectedError error
	}{
		{
			name: "successful stage volume",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "test-volume",
				StagingTargetPath: "/tmp/staging",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
				},
			},
			mockSetup: func(m *MockMounter) {
				devicePath := "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_test-volume"
				m.AttachedDevices[devicePath] = true
				m.FormattedDevices[devicePath] = "ext4"
			},
			expectedError: nil,
		},
		{
			name: "missing volume ID",
			req: &csi.NodeStageVolumeRequest{
				StagingTargetPath: "/tmp/staging",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
				},
			},
			expectedError: status.Error(codes.InvalidArgument, "NodeStageVolume Volume ID must be provided"),
		},
		{
			name: "missing staging target path",
			req: &csi.NodeStageVolumeRequest{
				VolumeId: "test-volume",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
				},
			},
			expectedError: status.Error(codes.InvalidArgument, "NodeStageVolume Staging Target Path must be provided"),
		},
		{
			name: "missing volume capability",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "test-volume",
				StagingTargetPath: "/tmp/staging",
			},
			expectedError: status.Error(codes.InvalidArgument, "NodeStageVolume Volume Capability must be provided"),
		},
		{
			name: "device not attached",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "test-volume",
				StagingTargetPath: "/tmp/staging",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
				},
			},
			mockSetup: func(m *MockMounter) {
				devicePath := "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_test-volume"
				m.AttachedDevices[devicePath] = false
				m.MountPoints[devicePath] = devicePath
			},
			expectedError: status.Error(codes.Internal, "error retrieving the attachement status \"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_test-volume\": device not attached"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMounter := NewMockMounter()
			if tt.mockSetup != nil {
				tt.mockSetup(mockMounter)
			}

			driver := &Driver{
				mounter:            mockMounter,
				log:                slog.New(slog.NewTextHandler(os.Stdout, nil)),
				validateAttachment: true,
			}

			resp, err := driver.NodeStageVolume(context.Background(), tt.req)
			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}

func TestNodeUnstageVolume(t *testing.T) {
	tests := []struct {
		name          string
		req           *csi.NodeUnstageVolumeRequest
		mockSetup     func(*MockMounter)
		expectedError error
	}{
		{
			name: "successful unstage volume",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "test-volume",
				StagingTargetPath: "/tmp/staging",
			},
			mockSetup: func(m *MockMounter) {
				m.MountPoints["/tmp/staging"] = "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_test-volume"
			},
			expectedError: nil,
		},
		{
			name: "missing volume ID",
			req: &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: "/tmp/staging",
			},
			expectedError: status.Error(codes.InvalidArgument, "NodeUnstageVolume Volume ID must be provided"),
		},
		{
			name: "missing staging target path",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId: "test-volume",
			},
			expectedError: status.Error(codes.InvalidArgument, "NodeUnstageVolume Staging Target Path must be provided"),
		},
		{
			name: "unmount error",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "test-volume",
				StagingTargetPath: "/tmp/staging",
			},
			mockSetup: func(m *MockMounter) {
				m.MountPoints["/tmp/staging"] = "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_test-volume"
				m.UnmountErrors["/tmp/staging"] = ErrUnmountFailed
			},
			expectedError: status.Error(codes.Internal, "unmount failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMounter := NewMockMounter()
			if tt.mockSetup != nil {
				tt.mockSetup(mockMounter)
			}

			driver := &Driver{
				mounter: mockMounter,
				log:     slog.New(slog.NewTextHandler(os.Stdout, nil)),
			}

			resp, err := driver.NodeUnstageVolume(context.Background(), tt.req)
			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}
