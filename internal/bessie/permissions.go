package bessie

import "github.com/purpose-robot/planet-express/internal/auth"

const (
	PermissionCredentialsRead     auth.Permission = "credentials:read"
	PermissionCredentialsWrite    auth.Permission = "credentials:write"
	PermissionNetworkDevicesRead  auth.Permission = "network-devices:read"
	PermissionNetworkDevicesWrite auth.Permission = "network-devices:write"
)
