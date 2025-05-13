package iaas

import (
	"context"
	"fmt"

	"github.com/thalassa-cloud/client-go/pkg/client"
)

const (
	VolumeTypeEndpoint = "/v1/volume-types"
)

// ListVolumeTypes lists all volume types.
func (c *Client) ListVolumeTypes(ctx context.Context) ([]VolumeType, error) {
	var volumeTypes []VolumeType
	req := c.R().SetResult(&volumeTypes)

	resp, err := c.Do(ctx, req, client.GET, VolumeTypeEndpoint)
	if err != nil {
		return nil, err
	}

	if err := c.Check(resp); err != nil {
		return nil, err
	}

	return volumeTypes, nil
}

// GetVolumeType gets a volume type by its identity.
func (c *Client) GetVolumeType(ctx context.Context, identity string) (*VolumeType, error) {
	var volumeType *VolumeType
	req := c.R().SetResult(&volumeType)

	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s", VolumeTypeEndpoint, identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return nil, err
	}
	return volumeType, nil
}
