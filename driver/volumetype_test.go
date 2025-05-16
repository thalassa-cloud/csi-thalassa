package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	iaas "github.com/thalassa-cloud/client-go/iaas"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetVolumeTypeByFilter(t *testing.T) {
	tests := []struct {
		name        string
		volumeTypes []iaas.VolumeType
		filter      func(volumeType iaas.VolumeType) bool
		want        string
		wantErr     error
	}{
		{
			name: "successful match",
			volumeTypes: []iaas.VolumeType{
				{Identity: "type1", Name: "block"},
				{Identity: "type2", Name: "block-premium"},
			},
			filter: func(vt iaas.VolumeType) bool {
				return vt.Name == "block"
			},
			want:    "type1",
			wantErr: nil,
		},
		{
			name: "no match",
			volumeTypes: []iaas.VolumeType{
				{Identity: "type1", Name: "block"},
				{Identity: "type2", Name: "block-premium"},
			},
			filter: func(vt iaas.VolumeType) bool {
				return vt.Name == "block-enterprise"
			},
			want:    "",
			wantErr: status.Error(codes.NotFound, "volume type not found"),
		},
		{
			name:        "empty volume types",
			volumeTypes: []iaas.VolumeType{},
			filter: func(vt iaas.VolumeType) bool {
				return vt.Name == "block"
			},
			want:    "",
			wantErr: status.Error(codes.NotFound, "volume type not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getVolumeTypeByFilter(tt.volumeTypes, tt.filter)
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetVolumeTypeByFilters(t *testing.T) {
	tests := []struct {
		name        string
		volumeTypes []iaas.VolumeType
		filters     []func(volumeType iaas.VolumeType) bool
		want        string
		wantErr     error
	}{
		{
			name: "first filter matches",
			volumeTypes: []iaas.VolumeType{
				{Identity: "type1", Name: "block"},
				{Identity: "type2", Name: "block-premium"},
			},
			filters: []func(volumeType iaas.VolumeType) bool{
				func(vt iaas.VolumeType) bool { return vt.Name == "block" },
				func(vt iaas.VolumeType) bool { return vt.Name == "block-premium" },
			},
			want:    "type1",
			wantErr: nil,
		},
		{
			name: "second filter matches",
			volumeTypes: []iaas.VolumeType{
				{Identity: "type1", Name: "block"},
				{Identity: "type2", Name: "block-premium"},
			},
			filters: []func(volumeType iaas.VolumeType) bool{
				func(vt iaas.VolumeType) bool { return vt.Name == "block-enterprise" },
				func(vt iaas.VolumeType) bool { return vt.Name == "block-premium" },
			},
			want:    "type2",
			wantErr: nil,
		},
		{
			name: "no filters match",
			volumeTypes: []iaas.VolumeType{
				{Identity: "type1", Name: "block"},
				{Identity: "type2", Name: "block-premium"},
			},
			filters: []func(volumeType iaas.VolumeType) bool{
				func(vt iaas.VolumeType) bool { return vt.Name == "block-enterprise" },
				func(vt iaas.VolumeType) bool { return vt.Name == "SAS" },
			},
			want:    "",
			wantErr: status.Error(codes.NotFound, "volume type not found"),
		},
		{
			name:        "empty volume types",
			volumeTypes: []iaas.VolumeType{},
			filters: []func(volumeType iaas.VolumeType) bool{
				func(vt iaas.VolumeType) bool { return vt.Name == "block" },
			},
			want:    "",
			wantErr: status.Error(codes.NotFound, "volume type not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getVolumeTypeByFilters(tt.volumeTypes, tt.filters...)
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
