package iaas

import (
	"context"
	"fmt"

	"github.com/thalassa-cloud/client-go/pkg/client"
)

const (
	LoadbalancerEndpoint = "/v1/loadbalancers"
)

// ListLoadbalancers lists all loadbalancers for a given organisation.
func (c *Client) ListLoadbalancers(ctx context.Context) ([]VpcLoadbalancer, error) {
	loadbalancers := []VpcLoadbalancer{}
	req := c.R().SetResult(&loadbalancers)

	resp, err := c.Do(ctx, req, client.GET, LoadbalancerEndpoint)
	if err != nil {
		return nil, err
	}

	if err := c.Check(resp); err != nil {
		return loadbalancers, err
	}
	return loadbalancers, nil
}

// GetLoadbalancer retrieves a specific loadbalancer by its identity.
func (c *Client) GetLoadbalancer(ctx context.Context, identity string) (*VpcLoadbalancer, error) {
	var loadbalancer *VpcLoadbalancer
	req := c.R().SetResult(&loadbalancer)
	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s", LoadbalancerEndpoint, identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return loadbalancer, err
	}
	return loadbalancer, nil
}

// CreateLoadbalancer creates a new loadbalancer.
func (c *Client) CreateLoadbalancer(ctx context.Context, create CreateLoadbalancer) (*VpcLoadbalancer, error) {
	var loadbalancer *VpcLoadbalancer
	req := c.R().
		SetBody(create).SetResult(&loadbalancer)

	resp, err := c.Do(ctx, req, client.POST, LoadbalancerEndpoint)
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return loadbalancer, err
	}
	return loadbalancer, nil
}

// UpdateLoadbalancer updates an existing loadbalancer.
func (c *Client) UpdateLoadbalancer(ctx context.Context, identity string, update UpdateLoadbalancer) (*VpcLoadbalancer, error) {
	var loadbalancer *VpcLoadbalancer
	req := c.R().
		SetBody(update).SetResult(&loadbalancer)

	resp, err := c.Do(ctx, req, client.PUT, fmt.Sprintf("%s/%s", LoadbalancerEndpoint, identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return loadbalancer, err
	}
	return loadbalancer, nil
}

// DeleteLoadbalancer deletes a specific loadbalancer by its identity.
func (c *Client) DeleteLoadbalancer(ctx context.Context, identity string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s", LoadbalancerEndpoint, identity))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}
