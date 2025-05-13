package iaas

import (
	"context"
	"fmt"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/thalassa-cloud/client-go/pkg/client"
)

const (
	MachineEndpoint = "/v1/machines"
)

// ListMachines lists all Machines for a given organisation.
func (c *Client) ListMachines(ctx context.Context) ([]Machine, error) {
	subnets := []Machine{}
	req := c.R().SetResult(&subnets)

	resp, err := c.Do(ctx, req, client.GET, MachineEndpoint)
	if err != nil {
		return nil, err
	}

	if err := c.Check(resp); err != nil {
		return subnets, err
	}
	return subnets, nil
}

// GetMachine retrieves a specific Machine by its identity.
func (c *Client) GetMachine(ctx context.Context, identity string) (*Machine, error) {
	var machine *Machine
	req := c.R().SetResult(&machine)
	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s", MachineEndpoint, identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return machine, err
	}
	return machine, nil
}

// CreateMachine creates a new Machine.
func (c *Client) CreateMachine(ctx context.Context, create CreateMachine) (*Machine, error) {
	var machine *Machine
	req := c.R().
		SetBody(create).SetResult(&machine)

	resp, err := c.Do(ctx, req, client.POST, MachineEndpoint)
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return machine, err
	}
	return machine, nil
}

// UpdateMachine updates an existing Machine.
func (c *Client) UpdateMachine(ctx context.Context, identity string, update UpdateMachine) (*Machine, error) {
	var machine *Machine
	req := c.R().
		SetBody(update).SetResult(&machine)

	resp, err := c.Do(ctx, req, client.PUT, fmt.Sprintf("%s/%s", MachineEndpoint, identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return machine, err
	}
	return machine, nil
}

// DeleteMachine deletes a specific Machine by its identity.
func (c *Client) DeleteMachine(ctx context.Context, identity string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s", MachineEndpoint, identity))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

// start,stop,restart
func (c *Client) MachineStart(ctx context.Context, identity string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/start", MachineEndpoint, identity))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) MachineStop(ctx context.Context, identity string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/stop", MachineEndpoint, identity))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) MachineRestart(ctx context.Context, identity string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/restart", MachineEndpoint, identity))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

// console
// This creates a new console for the machine and returns a websocket connection to the console.
func (c *Client) MachineConsole(ctx context.Context, identity string) (*websocket.Conn, error) {
	// The API endpoint for the console
	consoleEndpoint := fmt.Sprintf("%s/%s/console", MachineEndpoint, identity)
	endpoint := c.GetBaseURL() + consoleEndpoint
	// convert to websocket
	endpoint = strings.Replace(endpoint, "http://", "ws://", 1)
	endpoint = strings.Replace(endpoint, "https://", "wss://", 1)

	// Get the websocket connection directly from the console endpoint
	return c.DialWebsocket(ctx, endpoint)
}
