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

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
	"github.com/thalassa-cloud/client-go/pkg/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateSnapshot creates a new snapshot from a source volume.
func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot Name must be provided")
	}
	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot Source Volume ID must be provided")
	}
	log := d.log.With("req_name", req.GetName(), "req_source_volume_id", req.GetSourceVolumeId(), "req_parameters", req.GetParameters(), "method", "create_snapshot")
	log.Info("creating snapshot")

	snapshot, err := d.getOrCreateSnapshot(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if snapshot == nil {
		return nil, status.Error(codes.NotFound, "snapshot not found or not created")
	}
	log.With("snapshot_identity", snapshot.Identity).Info("waiting for snapshot to be ready")
	// wait for the snapshot to be ready
	if err := d.iaas.WaitUntilSnapshotIsAvailable(ctx, snapshot.Identity); err != nil {
		log.With("snapshot_identity", snapshot.Identity).Error("failed to wait for snapshot to be ready", "error", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.With("snapshot_identity", snapshot.Identity).Info("snapshot is ready")
	snapshot, err = d.iaas.GetSnapshot(ctx, snapshot.Identity)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "snapshot not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.With("snapshot_identity", snapshot.Identity).Info("mapping snapshot to CSI snapshot")
	mapped, err := mapToCSISnapshot(snapshot)
	if err != nil {
		log.With("snapshot_identity", snapshot.Identity).Error("failed to map snapshot to CSI snapshot", "error", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.CreateSnapshotResponse{
		Snapshot: mapped,
	}, nil
}

func (d *Driver) getOrCreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*iaas.Snapshot, error) {
	log := d.log.With("req_name", req.GetName(), "req_source_volume_id", req.GetSourceVolumeId(), "req_parameters", req.GetParameters(), "method", "get_or_create_snapshot")
	log.Info("getting or creating snapshot")

	// First try to find the snapshot by name
	snapshots, err := d.iaas.ListSnapshots(ctx, &iaas.ListSnapshotsRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   filters.FilterRegion,
				Value: d.region,
			},
			&filters.FilterKeyValue{
				Key:   filters.FilterKey("name"),
				Value: req.GetName(),
			},
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list snapshots: %s", err)
	}
	for _, snapshot := range snapshots {
		if snapshot.Name == req.GetName() {
			return &snapshot, nil
		}
	}

	// If not found by name, try to find by volume name
	volumes, err := d.iaas.ListVolumes(ctx, &iaas.ListVolumesRequest{
		Filters: []filters.Filter{
			&filters.FilterKeyValue{
				Key:   filters.FilterRegion,
				Value: d.region,
			},
			&filters.FilterKeyValue{
				Key:   filters.FilterKey("name"),
				Value: req.GetSourceVolumeId(),
			},
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list volumes: %s", err)
	}

	if len(volumes) == 0 {
		return nil, status.Errorf(codes.NotFound, "volume with name %q not found", req.GetSourceVolumeId())
	}

	if len(volumes) > 1 {
		return nil, status.Errorf(codes.Internal, "multiple volumes found with name %q", req.GetSourceVolumeId())
	}
	volume := volumes[0]

	volumeIdentity := volume.Identity
	log = log.With("resolved_volume_identity", volumeIdentity)
	log.Info("resolved volume identity for snapshot creation")

	labels := iaas.Labels{
		"csi.volume.id":                      req.GetSourceVolumeId(),
		"k8s.thalassa.cloud/csi-driver":      "true",
		"k8s.thalassa.cloud/csi-driver-name": d.name,
	}
	annotations := iaas.Annotations{
		"csi.snapshot.id":                req.GetName(),
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

	if d.clusterIdentity != "" {
		labels["k8s.thalassa.cloud/cluster-identity"] = d.clusterIdentity
	}

	snapshot, err := d.iaas.CreateSnapshot(ctx, iaas.CreateSnapshotRequest{
		Name:           req.GetName(),
		VolumeIdentity: volumeIdentity,
		Labels:         labels,
		Annotations:    annotations,
	})

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return snapshot, nil
}

// DeleteSnapshot deletes a snapshot.
func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	log := d.log.With("req_snapshot_id", req.GetSnapshotId(), "method", "delete_snapshot")
	log.Info("deleting snapshot")
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot Snapshot ID must be provided")
	}

	err := d.iaas.DeleteSnapshot(ctx, req.GetSnapshotId())
	if err != nil {
		if client.IsNotFound(err) {
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.Info("snapshot was deleted")
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots lists all snapshots on the storage system within the given parameters
func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	listResp := &csi.ListSnapshotsResponse{}

	log := d.log.With("snapshot_id", req.SnapshotId, "source_volume_id", req.SourceVolumeId, "max_entries", req.MaxEntries, "req_starting_token", req.StartingToken, "method", "list_snapshots")
	log.Info("listing snapshots")

	if req.SnapshotId != "" {
		snapshot, err := d.iaas.GetSnapshot(ctx, req.SnapshotId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		mapped, err := mapToCSISnapshot(snapshot)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		listResp.Entries = append(listResp.Entries, &csi.ListSnapshotsResponse_Entry{
			Snapshot: mapped,
		})
		listResp.NextToken = snapshot.Identity

		log.With("num_snapshot_entries", len(listResp.Entries)).Info("listing snapshots")
		return listResp, nil
	}

	requestFilters := []filters.Filter{
		&filters.FilterKeyValue{
			Key:   filters.FilterRegion,
			Value: d.region,
		},
	}

	if req.SourceVolumeId != "" {
		requestFilters = append(requestFilters, &filters.FilterKeyValue{
			Key:   filters.FilterKey("SourceVolume"),
			Value: req.SourceVolumeId,
		})
	}
	snapshots, err := d.iaas.ListSnapshots(ctx, &iaas.ListSnapshotsRequest{
		Filters: requestFilters,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, snapshot := range snapshots {
		mapped, err := mapToCSISnapshot(&snapshot)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		listResp.Entries = append(listResp.Entries, &csi.ListSnapshotsResponse_Entry{
			Snapshot: mapped,
		})
	}

	log.With("num_snapshot_entries", len(listResp.Entries)).Info("listing snapshots")
	return listResp, nil
}

func mapToCSISnapshot(snap *iaas.Snapshot) (*csi.Snapshot, error) {
	var sourceVolumeId string
	if snap.SourceVolume != nil {
		sourceVolumeId = snap.SourceVolume.Identity
	}

	var sizeBytes int64
	if snap.SizeGB != nil {
		sizeBytes = int64(*snap.SizeGB) * giB
	}

	return &csi.Snapshot{
		SnapshotId:     snap.Identity,
		SourceVolumeId: sourceVolumeId,
		SizeBytes:      sizeBytes,
		CreationTime:   timestamppb.New(snap.CreatedAt),
		ReadyToUse:     snap.Status == iaas.SnapshotStatusAvailable,
	}, nil
}
