package iaas

import (
	"context"
	"fmt"

	"github.com/thalassa-cloud/client-go/pkg/client"
)

// ListListeners lists all listeners for a specific loadbalancer.
func (c *Client) ListListeners(ctx context.Context, loadbalancerID string) ([]VpcLoadbalancerListener, error) {
	listeners := []VpcLoadbalancerListener{}
	req := c.R().SetResult(&listeners)

	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s/listeners", LoadbalancerEndpoint, loadbalancerID))
	if err != nil {
		return nil, err
	}

	if err := c.Check(resp); err != nil {
		return listeners, err
	}
	return listeners, nil
}

// GetListener retrieves a specific loadbalancer listener by its identity.
func (c *Client) GetListener(ctx context.Context, loadbalancerID string, listenerID string) (*VpcLoadbalancerListener, error) {
	var listener *VpcLoadbalancerListener
	req := c.R().SetResult(&listener)
	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s/listeners/%s", LoadbalancerEndpoint, loadbalancerID, listenerID))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return listener, err
	}
	return listener, nil
}

// CreateListener creates a new loadbalancer listener.
func (c *Client) CreateListener(ctx context.Context, loadbalancerID string, create CreateListener) (*VpcLoadbalancerListener, error) {
	var listener *VpcLoadbalancerListener
	req := c.R().
		SetBody(create).SetResult(&listener)

	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/listeners", LoadbalancerEndpoint, loadbalancerID))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return listener, err
	}
	return listener, nil
}

// UpdateListener updates an existing loadbalancer listener.
func (c *Client) UpdateListener(ctx context.Context, loadbalancerID string, listenerID string, update UpdateListener) (*VpcLoadbalancerListener, error) {
	var listener *VpcLoadbalancerListener
	req := c.R().
		SetBody(update).SetResult(&listener)

	resp, err := c.Do(ctx, req, client.PUT, fmt.Sprintf("%s/%s/listeners/%s", LoadbalancerEndpoint, loadbalancerID, listenerID))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return listener, err
	}
	return listener, nil
}

// DeleteListener deletes a specific loadbalancer listener by its identity.
func (c *Client) DeleteListener(ctx context.Context, loadbalancerID string, listenerID string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s/listeners/%s", LoadbalancerEndpoint, loadbalancerID, listenerID))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}
