package ssh

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/purpose-robot/planet-express/internal/bessie"
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

	conn, err := dialer.DialContext(
		dialCtx,
		"tcp",
		net.JoinHostPort(ipAddress.String(), strconv.Itoa(c.config.Port)),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", bessie.ErrDiscovererConnectionFailed, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	return nil
}

func (c *InventoryCollector) Collect(ctx context.Context, ipAddress netip.Addr, credential *bessie.Credential) (bessie.NetworkDeviceInventory, error) {
	if err := ctx.Err(); err != nil {
		return bessie.NetworkDeviceInventory{}, err
	}

	if !ipAddress.IsValid() {
		return bessie.NetworkDeviceInventory{}, fmt.Errorf("%w: invalid IP address", bessie.ErrDiscovererInvalidOptions)
	}

	if credential == nil {
		return bessie.NetworkDeviceInventory{}, fmt.Errorf("%w: credential cannot be nil", bessie.ErrDiscovererInvalidOptions)
	}

	if err := c.ensureReachable(ctx, ipAddress); err != nil {
		return bessie.NetworkDeviceInventory{}, err
	}

	client, err := NewClient(ipAddress.String(), credential, c.config)
	if err != nil {
		return bessie.NetworkDeviceInventory{}, err
	}

	if err := client.Open(ctx); err != nil {
		return bessie.NetworkDeviceInventory{}, err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
		defer cancel()

		_ = client.Close(closeCtx)
	}()

	return client.CollectInventory(ctx)
}
