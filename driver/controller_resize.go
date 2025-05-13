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
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/thalassa-cloud/client-go/iaas"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ControllerExpandVolume is called from the resizer to increase the volume size.
func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	if strings.TrimSpace(volumeId) == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume volume ID missing in request")
	}

	resizeBytes, err := getStorageSizeFromCapacityRange(req.GetCapacityRange())
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "ControllerExpandVolume invalid capacity range: %v", err)
	}
	resizeGigaBytes := resizeBytes / giB

	log := d.log.With("volume_id", volumeId, "method", "controller_expand_volume")
	log.Info("expanding volume")

	volume, err := d.iaas.GetVolume(ctx, volumeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ControllerExpandVolume could not retrieve existing volume: %v", err)
	}

	if isVolumeSizeEuqalOrLargerThanRequested(volume, resizeGigaBytes) {
		log.With("current_volume_size", volume.Size, "requested_volume_size", resizeGigaBytes).Info("no resize necessary because API volume size is equal or larger than requested volume size")
		return &csi.ControllerExpandVolumeResponse{CapacityBytes: int64(volume.Size) * giB, NodeExpansionRequired: true}, nil
	}

	if _, err := d.iaas.UpdateVolume(ctx, volumeId, iaas.UpdateVolume{
		Name:             volume.Name,
		Description:      volume.Description,
		Labels:           volume.Labels,
		Annotations:      volume.Annotations,
		Size:             int(resizeGigaBytes),
		DeleteProtection: volume.DeleteProtection,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "cannot resize volume %s: %s", volumeId, err.Error())
	}

	log = log.With("new_volume_size", resizeGigaBytes)
	log.Info("volume was resized")

	nodeExpansionRequired := true
	if req.GetVolumeCapability() != nil {
		if _, ok := req.GetVolumeCapability().GetAccessType().(*csi.VolumeCapability_Block); ok {
			log.Debug("node expansion is not required for block volumes")
			nodeExpansionRequired = false
		}
	}
	return &csi.ControllerExpandVolumeResponse{CapacityBytes: resizeGigaBytes * giB, NodeExpansionRequired: nodeExpansionRequired}, nil
}

// isVolumeSizeEuqalOrLargerThanRequested checks if the volume size is equal or larger than the requested size
func isVolumeSizeEuqalOrLargerThanRequested(volume *iaas.Volume, requestedSize int64) bool {
	return volume.Size >= int(requestedSize)
}
