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
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
)

func TestGetStorageSizeFromCapacityRange(t *testing.T) {
	tests := []struct {
		name          string
		capRange      *csi.CapacityRange
		expectedSize  int64
		expectedError string
	}{
		{
			name:          "nil capacity range returns default size",
			capRange:      nil,
			expectedSize:  defaultVolumeSizeInBytes,
			expectedError: "",
		},
		{
			name: "empty capacity range returns default size",
			capRange: &csi.CapacityRange{
				RequiredBytes: 0,
				LimitBytes:    0,
			},
			expectedSize:  defaultVolumeSizeInBytes,
			expectedError: "",
		},
		{
			name: "required bytes less than minimum returns minimum size",
			capRange: &csi.CapacityRange{
				RequiredBytes: minimumVolumeSizeInBytes - 1,
				LimitBytes:    0,
			},
			expectedSize:  minimumVolumeSizeInBytes,
			expectedError: "",
		},
		{
			name: "required bytes greater than maximum returns error",
			capRange: &csi.CapacityRange{
				RequiredBytes: maximumVolumeSizeInBytes + 1,
				LimitBytes:    0,
			},
			expectedSize:  0,
			expectedError: "required (16Ti) can not exceed maximum supported volume size (16Ti)",
		},
		{
			name: "limit bytes less than minimum returns error",
			capRange: &csi.CapacityRange{
				RequiredBytes: 0,
				LimitBytes:    minimumVolumeSizeInBytes - 1,
			},
			expectedSize:  0,
			expectedError: "limit (1024Mi) can not be less than minimum supported volume size (1Gi)",
		},
		{
			name: "limit bytes greater than maximum returns error",
			capRange: &csi.CapacityRange{
				RequiredBytes: 0,
				LimitBytes:    maximumVolumeSizeInBytes + 1,
			},
			expectedSize:  0,
			expectedError: "limit (16Ti) can not exceed maximum supported volume size (16Ti)",
		},
		{
			name: "limit less than required returns error",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * 1024 * 1024 * 1024, // 100 GB
				LimitBytes:    50 * 1024 * 1024 * 1024,  // 50 GB
			},
			expectedSize:  0,
			expectedError: "limit (50Gi) can not be less than required (100Gi) size",
		},
		{
			name: "required equals limit returns required size",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * 1024 * 1024 * 1024, // 100 GB
				LimitBytes:    100 * 1024 * 1024 * 1024, // 100 GB
			},
			expectedSize:  100 * 1024 * 1024 * 1024,
			expectedError: "",
		},
		{
			name: "only required bytes set returns required size",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * 1024 * 1024 * 1024, // 100 GB
				LimitBytes:    0,
			},
			expectedSize:  100 * 1024 * 1024 * 1024,
			expectedError: "",
		},
		{
			name: "only limit bytes set returns limit size",
			capRange: &csi.CapacityRange{
				RequiredBytes: 0,
				LimitBytes:    100 * 1024 * 1024 * 1024, // 100 GB
			},
			expectedSize:  100 * 1024 * 1024 * 1024,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := getStorageSizeFromCapacityRange(tt.capRange)
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Equal(t, tt.expectedError, err.Error())
				require.Equal(t, int64(0), size)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedSize, size)
			}
		})
	}
}
