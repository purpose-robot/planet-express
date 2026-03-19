package ssh

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/purpose-robot/planet-express/internal/cisco"
)

type InventoryCollector struct {
	config Config
}

func NewInventoryCollector(config Config) *InventoryCollector {
	return &InventoryCollector{
		config: config,
	}
}

func (c *InventoryCollector) ensureReachable(ctx context.Context, ipAddress netip.Addr) error {
	dialCtx, cancel := context.WithTimeout(ctx, c.config.TCPDialTimeout)
	defer cancel()

	dialer := net.Dialer{}

	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(ipAddress.String(), strconv.Itoa(c.config.Port)))
	if err != nil {
		return fmt.Errorf("%w: %w", cisco.ErrDiscovererConnectionFailed, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	return nil
}

func (c *InventoryCollector) Collect(ctx context.Context, ipAddress netip.Addr, credential *cisco.Credential) (cisco.NetworkDeviceInventory, error) {
	if err := ctx.Err(); err != nil {
		return cisco.NetworkDeviceInventory{}, err
	}

	if !ipAddress.IsValid() {
		return cisco.NetworkDeviceInventory{}, fmt.Errorf("%w: invalid IP address", cisco.ErrDiscovererInvalidOptions)
	}

	if credential == nil {
		return cisco.NetworkDeviceInventory{}, fmt.Errorf("%w: credential cannot be nil", cisco.ErrDiscovererInvalidOptions)
	}

	if err := c.ensureReachable(ctx, ipAddress); err != nil {
		return cisco.NetworkDeviceInventory{}, err
	}

	client, err := NewClient(ipAddress.String(), credential, c.config)
	if err != nil {
		return cisco.NetworkDeviceInventory{}, err
	}

	if err := client.Open(ctx); err != nil {
		return cisco.NetworkDeviceInventory{}, err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
		defer cancel()

		_ = client.Close(closeCtx)
	}()

	return client.CollectInventory(ctx)
}
