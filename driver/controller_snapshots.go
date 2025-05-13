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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	return nil, status.Error(codes.Unimplemented, "snapshot is not supported")
}

// DeleteSnapshot deletes a snapshot.
func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	log := d.log.With("req_snapshot_id", req.GetSnapshotId(), "method", "delete_snapshot")
	log.Info("deleting snapshot")
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot Snapshot ID must be provided")
	}
	return nil, status.Error(codes.Unimplemented, "snapshot is not supported")
}

// ListSnapshots lists all snapshots on the storage system within the given parameters
func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	listResp := &csi.ListSnapshotsResponse{}

	log := d.log.With("snapshot_id", req.SnapshotId, "source_volume_id", req.SourceVolumeId, "max_entries", req.MaxEntries, "req_starting_token", req.StartingToken, "method", "list_snapshots")
	log.Info("listing snapshots")
	return listResp, nil
}
