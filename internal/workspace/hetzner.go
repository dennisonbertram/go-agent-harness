package workspace

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

const (
	hetznerDefaultImage      = "ubuntu-24.04"
	hetznerDefaultServerType = "cx22"
	hetznerPollInterval      = 3 * time.Second
	hetznerProvisionTimeout  = 5 * time.Minute
)

// HetznerProvider implements VMProvider using the Hetzner Cloud API.
type HetznerProvider struct {
	client *hcloud.Client
}

// NewHetznerProvider creates a HetznerProvider authenticated with the given API key.
func NewHetznerProvider(apiKey string) *HetznerProvider {
	client := hcloud.NewClient(hcloud.WithToken(apiKey))
	return &HetznerProvider{client: client}
}

// Create provisions a new Hetzner Cloud server and waits until it is running.
// It polls every 3 seconds with a 5-minute timeout. The returned VM contains
// the server's string ID and its public IPv4 address.
func (p *HetznerProvider) Create(ctx context.Context, opts VMCreateOpts) (*VM, error) {
	imageName := opts.ImageName
	if imageName == "" {
		imageName = hetznerDefaultImage
	}
	serverType := opts.ServerType
	if serverType == "" {
		serverType = hetznerDefaultServerType
	}

	createOpts := hcloud.ServerCreateOpts{
		Name:       opts.Name,
		ServerType: &hcloud.ServerType{Name: serverType},
		Image:      &hcloud.Image{Name: imageName},
		UserData:   opts.UserData,
	}

	result, _, err := p.client.Server.Create(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("hetzner: server create: %w", err)
	}

	server := result.Server

	// Poll until the server reaches running status.
	deadline := time.Now().Add(hetznerProvisionTimeout)
	for {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("hetzner: context cancelled while waiting for server: %w", ctx.Err())
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("hetzner: timed out waiting for server %d to reach running status", server.ID)
		}

		updated, _, err := p.client.Server.GetByID(ctx, server.ID)
		if err != nil {
			return nil, fmt.Errorf("hetzner: polling server status: %w", err)
		}
		if updated == nil {
			return nil, fmt.Errorf("hetzner: server %d disappeared during provisioning", server.ID)
		}

		if updated.Status == hcloud.ServerStatusRunning {
			return &VM{
				ID:       strconv.FormatInt(updated.ID, 10),
				PublicIP: updated.PublicNet.IPv4.IP.String(),
				Status:   string(updated.Status),
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("hetzner: context cancelled while waiting for server: %w", ctx.Err())
		case <-time.After(hetznerPollInterval):
		}
	}
}

// Delete terminates the Hetzner Cloud server with the given string ID.
// The ID must be the decimal string representation of the server's integer ID.
func (p *HetznerProvider) Delete(ctx context.Context, id string) error {
	serverID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("hetzner: invalid server ID %q: %w", id, err)
	}

	server, _, err := p.client.Server.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("hetzner: get server %d: %w", serverID, err)
	}
	if server == nil {
		// Already gone — treat as success.
		return nil
	}

	_, err = p.client.Server.Delete(ctx, server)
	if err != nil {
		return fmt.Errorf("hetzner: delete server %d: %w", serverID, err)
	}
	return nil
}
