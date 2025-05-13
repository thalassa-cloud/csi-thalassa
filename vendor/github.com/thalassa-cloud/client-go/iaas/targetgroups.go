package iaas

import (
	"context"
	"fmt"

	"github.com/thalassa-cloud/client-go/pkg/client"
)

const (
	TargetGroupEndpoint = "/v1/loadbalancer-target-groups"
)

// ListTargetGroups lists all loadbalancer target groups for a given organisation.
func (c *Client) ListTargetGroups(ctx context.Context) ([]VpcLoadbalancerTargetGroup, error) {
	targetGroups := []VpcLoadbalancerTargetGroup{}
	req := c.R().SetResult(&targetGroups)

	resp, err := c.Do(ctx, req, client.GET, TargetGroupEndpoint)
	if err != nil {
		return nil, err
	}

	if err := c.Check(resp); err != nil {
		return targetGroups, err
	}
	return targetGroups, nil
}

// GetTargetGroup retrieves a specific loadbalancer target group by its identity.
func (c *Client) GetTargetGroup(ctx context.Context, identity string) (*VpcLoadbalancerTargetGroup, error) {
	var targetGroup *VpcLoadbalancerTargetGroup
	req := c.R().SetResult(&targetGroup)
	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s", TargetGroupEndpoint, identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return targetGroup, err
	}
	return targetGroup, nil
}

// CreateTargetGroup creates a new loadbalancer target group.
func (c *Client) CreateTargetGroup(ctx context.Context, create CreateTargetGroup) (*VpcLoadbalancerTargetGroup, error) {
	var targetGroup *VpcLoadbalancerTargetGroup
	req := c.R().
		SetBody(create).SetResult(&targetGroup)

	resp, err := c.Do(ctx, req, client.POST, TargetGroupEndpoint)
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return targetGroup, err
	}
	return targetGroup, nil
}

// UpdateTargetGroup updates an existing loadbalancer target group.
func (c *Client) UpdateTargetGroup(ctx context.Context, identity string, update UpdateTargetGroup) (*VpcLoadbalancerTargetGroup, error) {
	var targetGroup *VpcLoadbalancerTargetGroup
	req := c.R().
		SetBody(update).SetResult(&targetGroup)

	resp, err := c.Do(ctx, req, client.PUT, fmt.Sprintf("%s/%s", TargetGroupEndpoint, identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return targetGroup, err
	}
	return targetGroup, nil
}

// DeleteTargetGroup deletes a specific loadbalancer target group by its identity.
func (c *Client) DeleteTargetGroup(ctx context.Context, identity string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s", TargetGroupEndpoint, identity))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

// AttachServerToTargetGroup attaches a server to a target group.
func (c *Client) AttachServerToTargetGroup(ctx context.Context, targetGroupID string, attachment AttachTargetRequest) (*LoadbalancerTargetGroupAttachment, error) {
	var result *LoadbalancerTargetGroupAttachment
	req := c.R().
		SetBody(attachment).SetResult(&result)

	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/attachments", TargetGroupEndpoint, targetGroupID))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return result, err
	}
	return result, nil
}

// DetachServerFromTargetGroup detaches a server from a target group.
func (c *Client) DetachServerFromTargetGroup(ctx context.Context, targetGroupID string, attachmentID string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s/attachments/%s", TargetGroupEndpoint, targetGroupID, attachmentID))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}
