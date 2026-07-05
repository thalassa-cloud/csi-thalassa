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
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockAttachmentValidator struct {
	symlink string
	state   []byte
}

func (m *mockAttachmentValidator) readFile(name string) ([]byte, error) {
	return m.state, nil
}

func (m *mockAttachmentValidator) evalSymlinks(path string) (string, error) {
	return m.symlink, nil
}

func TestMounterIsAttached(t *testing.T) {
	tests := []struct {
		name          string
		state         []byte
		expectedError string
	}{
		{
			name:  "running state with trailing newline",
			state: []byte("running\n"),
		},
		{
			name:  "running state without trailing newline",
			state: []byte("running"),
		},
		{
			name:          "device not running",
			state:         []byte("blocked\n"),
			expectedError: `error comparing the state file content, expected: running, got: "blocked"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mounter{
				log: slog.New(slog.NewTextHandler(os.Stdout, nil)),
				attachmentValidator: &mockAttachmentValidator{
					symlink: "/sys/class/block/sdb",
					state:   tt.state,
				},
			}

			err := m.IsAttached("/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_test-volume")
			if tt.expectedError != "" {
				require.EqualError(t, err, tt.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}
