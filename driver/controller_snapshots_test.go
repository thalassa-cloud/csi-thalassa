package driver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thalassa-cloud/client-go/iaas"
)

func TestSnapshotSourceVolumeIdentity(t *testing.T) {
	sourceVolumeID := "vol-identity"
	tests := []struct {
		name     string
		snapshot iaas.Snapshot
		want     string
	}{
		{
			name: "prefers source volume object identity",
			snapshot: iaas.Snapshot{
				SourceVolume:   &iaas.Volume{Identity: "vol-from-object"},
				SourceVolumeId: &sourceVolumeID,
			},
			want: "vol-from-object",
		},
		{
			name: "falls back to source volume id field",
			snapshot: iaas.Snapshot{
				SourceVolumeId: &sourceVolumeID,
			},
			want: "vol-identity",
		},
		{
			name:     "returns empty when source volume is unknown",
			snapshot: iaas.Snapshot{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, snapshotSourceVolumeIdentity(&tt.snapshot))
		})
	}
}

func TestSnapshotSourceVolumeMatches(t *testing.T) {
	sourceVolumeID := "vol-identity"
	tests := []struct {
		name                         string
		requestedSourceVolumeID      string
		resolvedSourceVolumeIdentity string
		snapshot                     iaas.Snapshot
		want                         bool
	}{
		{
			name:                         "matches resolved source volume identity",
			requestedSourceVolumeID:      "vol-name",
			resolvedSourceVolumeIdentity: "vol-identity",
			snapshot: iaas.Snapshot{
				SourceVolumeId: &sourceVolumeID,
			},
			want: true,
		},
		{
			name:                         "rejects different resolved source volume identity",
			requestedSourceVolumeID:      "other-volume",
			resolvedSourceVolumeIdentity: "other-identity",
			snapshot: iaas.Snapshot{
				SourceVolumeId: &sourceVolumeID,
			},
			want: false,
		},
		{
			name:                         "matches requested source volume identity without resolution",
			requestedSourceVolumeID:      "vol-identity",
			resolvedSourceVolumeIdentity: "",
			snapshot: iaas.Snapshot{
				SourceVolumeId: &sourceVolumeID,
			},
			want: true,
		},
		{
			name:                         "matches requested source volume name without resolution",
			requestedSourceVolumeID:      "vol-name",
			resolvedSourceVolumeIdentity: "",
			snapshot: iaas.Snapshot{
				SourceVolume: &iaas.Volume{
					Identity: "vol-identity",
					Name:     "vol-name",
				},
			},
			want: true,
		},
		{
			name:                         "rejects unknown existing source volume",
			requestedSourceVolumeID:      "vol-identity",
			resolvedSourceVolumeIdentity: "vol-identity",
			snapshot:                     iaas.Snapshot{},
			want:                         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshotSourceVolumeMatches(
				tt.requestedSourceVolumeID,
				tt.resolvedSourceVolumeIdentity,
				&tt.snapshot,
			)
			require.Equal(t, tt.want, got)
		})
	}
}
