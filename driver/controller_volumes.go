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
	"fmt"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
	"github.com/thalassa-cloud/client-go/pkg/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateVolume creates a new volume from the given request
func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}

	if len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}

	if violations := validateCapabilities(req.VolumeCapabilities); len(violations) > 0 {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("volume capabilities cannot be satisified: %s", strings.Join(violations, "; ")))
	}

	size, err := getStorageSizeFromCapacityRange(req.CapacityRange)
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "invalid capacity range: %v", err)
	}

	if req.AccessibilityRequirements != nil {
		for _, t := range req.AccessibilityRequirements.Requisite {
			region, ok := t.Segments["region"]
			if !ok {
				continue // nothing to do
			}

			if region != d.region {
				return nil, status.Errorf(codes.ResourceExhausted, "volume can be only created in region: %q, got: %q", d.region, region)
			}
		}
	}

	volumeName := req.Name

	volumeIdentity := req.Parameters["volume-identity"]
	if volumeIdentity == "" {
		volumeIdentity = volumeName
	}

	log := d.log.With("volume_name", volumeName, "storage_size_giga_bytes", size/giB, "method", "create_volume", "volume_capabilities", req.VolumeCapabilities)
	log.Info("creating volume")

	log.With("volume_identity", volumeIdentity).Info("getting volume to check if it already exists")
	// get volume first, if it's created do no thing
	volume, err := d.iaas.GetVolume(ctx, volumeIdentity)
	if err != nil && !client.IsNotFound(err) {
		log.Error("failed to get volume to check if it already exists", "error", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if volume != nil {
		log.With("volume_identity", volume.Identity).Info("volume already created")

		if int64(volume.Size)*giB != size {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("invalid option requested size: %d", size))
		}

		log.Info("volume already created")
		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:      volume.Identity,
				CapacityBytes: int64(volume.Size) * giB,
			},
		}, nil
	}

	volumeTypeParam := req.Parameters["volume-type"]
	if volumeTypeParam == "" {
		volumeTypeParam = "block"
	}

	volumeTypes, err := d.iaas.ListVolumeTypes(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	volumeTypeIdentity, err := getVolumeTypeByFilters(volumeTypes,
		func(volumeType iaas.VolumeType) bool {
			return volumeType.Identity == volumeTypeParam
		},
		func(volumeType iaas.VolumeType) bool {
			return strings.EqualFold(volumeType.Name, volumeTypeParam)
		},
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if volumeTypeIdentity == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume type: %q: volume type not found", volumeTypeParam)
	}

	labels := iaas.Labels{
		"k8s.thalassa.cloud/csi-driver":      "true",
		"k8s.thalassa.cloud/csi-driver-name": d.name,
	}
	annotations := iaas.Annotations{
		"k8s.thalassa.cloud/description": "Provisioned by Thalassa CSI driver",
	}
	for k, v := range d.CustomLabels {
		if _, ok := labels[k]; !ok {
			labels[k] = v
		}
	}
	for k, v := range d.CustomAnnotations {
		if _, ok := annotations[k]; !ok {
			annotations[k] = v
		}
	}

	volumeReq := iaas.CreateVolume{
		Name:                volumeName,
		CloudRegionIdentity: d.region,
		Description:         createdByThalassaCSI,
		Size:                int(size / giB),
		VolumeTypeIdentity:  volumeTypeIdentity,
		Labels:              labels,
		Annotations:         annotations,
	}

	if d.clusterIdentity != "" {
		volumeReq.Labels["k8s.thalassa.cloud/cluster-identity"] = d.clusterIdentity
	}

	contentSource := req.GetVolumeContentSource()
	if contentSource != nil && contentSource.GetSnapshot() != nil {
		snapshotID := contentSource.GetSnapshot().GetSnapshotId()
		if snapshotID == "" {
			return nil, status.Error(codes.InvalidArgument, "snapshot ID is empty")
		}

		log.With("snapshot_id", snapshotID).Info("getting snapshot for restore")

		_, err = d.iaas.GetSnapshot(ctx, snapshotID)
		if err != nil {
			if client.IsNotFound(err) {
				return nil, status.Error(codes.NotFound, "snapshot not found for restore")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		log.With("snapshot_id", snapshotID).Info("using snapshot to create volume")

		volumeReq.RestoreFromSnapshotId = &snapshotID
	}

	log.With("volume_req", volumeReq).Info("creating volume")
	vol, err := d.iaas.CreateVolume(ctx, volumeReq)
	if err != nil {
		if volumeReq.RestoreFromSnapshotId != nil {
			log.With("snapshot_id", *volumeReq.RestoreFromSnapshotId).Warn("failed to create volume from snapshot")
			if client.IsNotFound(err) {
				return nil, status.Error(codes.NotFound, "snapshot not found for restore")
			}
		} else {
			log.Error("failed to create volume", "error", err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	createVolume := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.Identity,
			CapacityBytes: size,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						"region": d.region,
					},
				},
			},
			ContentSource: contentSource,
		},
	}

	log.With("response", createVolume).Info("volume was created")
	return createVolume, nil
}

// DeleteVolume deletes the given volume. The function is idempotent.
func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	log := d.log.With("volume_id", req.VolumeId, "method", "delete_volume")
	log.Info("deleting volume")

	err := d.iaas.DeleteVolume(ctx, req.VolumeId)
	if err != nil {
		if client.IsNotFound(err) {
			// we assume it's deleted already for idempotency
			log.With("error", err).Warn("assuming volume is deleted because it does not exist")
			return &csi.DeleteVolumeResponse{}, nil
		}
		log.Error("failed to delete volume", "error", err)
		return nil, err
	}
	log.Info("volume was deleted")
	return &csi.DeleteVolumeResponse{}, nil
}

// ListVolumes returns a list of all requested volumes
func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	maxEntries := req.MaxEntries
	if maxEntries == 0 && d.defaultVolumesPageSize > 0 {
		maxEntries = int32(d.defaultVolumesPageSize)
	}

	log := d.log.With("max_entries", req.MaxEntries, "effective_max_entries", maxEntries, "req_starting_token", req.StartingToken, "method", "list_volumes")
	log.Info("list volumes called")

	volumes, err := d.iaas.ListVolumes(ctx, &iaas.ListVolumesRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   filters.FilterRegion,
				Value: d.region,
			},
			// &filters.LabelFilter{
			// 	MatchLabels: map[string]string{
			// 		// "csi-driver": "thalassa",
			// 	},
			// },
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var entries []*csi.ListVolumesResponse_Entry
	for _, vol := range volumes {
		attachedMachinesIdentities := make([]string, 0, len(vol.Attachments))
		for _, attachment := range vol.Attachments {
			attachedMachinesIdentities = append(attachedMachinesIdentities, attachment.AttachedToIdentity)
		}

		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      vol.Identity,
				CapacityBytes: int64(vol.Size) * giB,
			},
			Status: &csi.ListVolumesResponse_VolumeStatus{
				PublishedNodeIds: attachedMachinesIdentities,
			},
		})
	}

	resp := &csi.ListVolumesResponse{
		Entries: entries,
	}

	log.With("num_volume_entries", len(resp.Entries)).Info("listing volumes")
	return resp, nil
}

// ControllerGetVolume gets a specific volume.
// The call is used for the CSI health check feature
// (https://github.com/kubernetes/enhancements/pull/1077) which we do not
// support yet.
func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerModifyVolume
func (d *Driver) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
