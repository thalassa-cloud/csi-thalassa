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
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
	"github.com/thalassa-cloud/client-go/pkg/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
)

// ControllerPublishVolume attaches the given volume to the node
func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}

	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Node ID must be provided")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume capability must be provided")
	}

	if req.Readonly {
		return nil, status.Error(codes.AlreadyExists, "read only Volumes are not supported")
	}

	log := d.log.With("volume_id", req.VolumeId, "node_id", req.NodeId, "method", "controller_publish_volume")
	log.Info("controller publish volume called")

	// check if volume exist before trying to attach it
	vol, err := d.iaas.GetVolume(ctx, req.VolumeId)
	if err != nil {
		if err == client.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "volume %q does not exist", req.VolumeId)
		}
		return nil, err
	}

	if d.kubeConfig != "" {
		// we construct a kubernetes client to get the node name
		providerID, err := d.getNodeMachineIdentity(ctx, req.NodeId)
		if err != nil {
			return nil, err
		}
		req.NodeId = providerID
	}

	// nodeName := req.NodeId
	// convert nodeName to provider id, as that is the machine identity we can use in the API

	attachToIdentity := req.NodeId
	// check if machine exist before trying to attach the volume to the machine
	_, err = d.iaas.GetMachine(ctx, req.NodeId)
	if err != nil {
		if err == client.ErrNotFound {
			// fallback to the node name
			machines, err := d.iaas.ListMachines(ctx, &iaas.ListMachinesRequest{})
			if err != nil {
				return nil, err
			}
			found := false
			for _, machine := range machines {
				if machine.Name == req.NodeId || machine.Identity == req.NodeId || machine.Slug == req.NodeId {
					attachToIdentity = machine.Identity
					found = true
					break
				}
			}
			if !found {
				return nil, status.Errorf(codes.NotFound, "machine %q does not exist", req.NodeId)
			}
		} else {
			return nil, err
		}
	}

	attachedToMachine := ""
	for _, attachment := range vol.Attachments {
		attachedToMachine = attachment.AttachedToIdentity
		if attachment.AttachedToIdentity == attachToIdentity {
			log.Info("volume is already attached")
			return &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{
					d.publishInfoVolumeName: vol.Name,
				},
			}, nil
		}
	}

	// machine is attached to a different node, return an error
	if attachedToMachine != "" {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q is attached to the wrong machine (%q), detach the volume to fix it", req.VolumeId, attachedToMachine)
	}

	// attach the volume to the correct node
	_, err = d.iaas.AttachVolume(ctx, req.VolumeId, iaas.AttachVolumeRequest{
		ResourceIdentity: attachToIdentity,
		ResourceType:     "cloud_virtual_machine",
	})
	if err != nil {
		return nil, err
	}

	log.Info("waiting until volume is attached")
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		vol, err := d.iaas.GetVolume(ctx, req.VolumeId)
		if err != nil {
			return false, fmt.Errorf("error getting volume: %w", err)
		}
		log.Info("volume status", "status", vol.Status)
		if strings.EqualFold(vol.Status, "attached") {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to attach volume: %w", err)
	}

	log.Info("volume was attached")
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			d.publishInfoVolumeName: vol.Name,
		},
	}, nil
}

// ControllerUnpublishVolume deattaches the given volume from the node
func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnpublishVolume Volume ID must be provided")
	}

	log := d.log.With("volume_id", req.VolumeId, "node_id", req.NodeId, "method", "controller_unpublish_volume")
	log.Info("controller unpublish volume called")

	// check if volume exist before trying to detach it
	_, err := d.iaas.GetVolume(ctx, req.VolumeId)
	if err != nil {
		if err == client.ErrNotFound {
			log.Info("assuming volume is detached because it does not exist")
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, err
	}

	if d.kubeConfig != "" {
		// we construct a kubernetes client to get the node name
		providerID, err := d.getNodeMachineIdentity(ctx, req.NodeId)
		if err != nil {
			return nil, err
		}
		req.NodeId = providerID
	}

	// nodeName := req.NodeId
	// convert nodeName to provider id, as that is the machine identity we can use in the API
	// We need to do this, because we currently have no other way to determine the provider id for the node (no metadata service exposed atm)

	attachToIdentity := req.NodeId
	// check if machine exists before trying to detach the volume from the machine
	_, err = d.iaas.GetMachine(ctx, attachToIdentity)
	if err != nil {
		if err == client.ErrNotFound {
			// fallback to the node name
			machines, err := d.iaas.ListMachines(ctx, &iaas.ListMachinesRequest{
				Filters: []filters.Filter{
					&filters.FilterKeyValue{
						Key:   filters.FilterRegion,
						Value: d.region,
					},
					&filters.FilterKeyValue{
						Key:   filters.FilterVpcIdentity,
						Value: d.vpc,
					},
				},
			})
			if err != nil {
				return nil, err
			}
			found := false
			for _, machine := range machines {
				if machine.Name == req.NodeId || machine.Identity == req.NodeId || machine.Slug == req.NodeId {
					attachToIdentity = machine.Identity
					found = true
					break
				}
			}
			if !found {
				return &csi.ControllerUnpublishVolumeResponse{}, nil
			}
		} else {
			return nil, err
		}
	}

	if err = d.iaas.DetachVolume(ctx, req.VolumeId, iaas.DetachVolumeRequest{
		ResourceIdentity: attachToIdentity,
		ResourceType:     "cloud_virtual_machine",
	}); err != nil {
		return nil, err
	}

	log.Info("waiting until volume is detached")
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		vol, err := d.iaas.GetVolume(ctx, req.VolumeId)
		if err != nil {
			return false, fmt.Errorf("error getting volume: %w", err)
		}
		log.Info("volume status", "status", vol.Status)
		if strings.EqualFold(vol.Status, "available") {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to detach volume: %w", err)
	}

	log.Info("volume was detached")
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}
