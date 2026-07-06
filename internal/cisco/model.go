package cisco

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/errorz"
)

type Credential struct {
	id          uuid.UUID
	createdAt   time.Time
	updatedAt   time.Time
	userID      *uuid.UUID
	username    string
	password    string
	authMethod  AuthMethod
	description *string
	lastUsedAt  *time.Time
}

type AuthMethod string

const (
	AuthMethodLocal  AuthMethod = "local"
	AuthMethodRemote AuthMethod = "remote"
)

func (c *Credential) ID() uuid.UUID {
	return c.id
}

func (c *Credential) CreatedAt() time.Time {
	return c.createdAt
}

func (c *Credential) UpdatedAt() time.Time {
	return c.updatedAt
}

func (c *Credential) UserID() *uuid.UUID {
	return c.userID
}

func (c *Credential) Username() string {
	return c.username
}

func (c *Credential) Password() string {
	return c.password
}

func (c *Credential) AuthMethod() AuthMethod {
	return c.authMethod
}

func (c *Credential) Description() *string {
	return c.description
}

func (c *Credential) LastUsedAt() *time.Time {
	return c.lastUsedAt
}

func (c *Credential) AccessibleBy(userID uuid.UUID) bool {
	if c.authMethod == AuthMethodLocal {
		return true
	}

	return c.userID != nil && *c.userID == userID
}

func NewCredential(userID *uuid.UUID, username, password, authMethod string, description *string) (*Credential, error) {
	parsedAuthMethod, err := ParseAuthMethod(authMethod)
	if err != nil {
		return nil, err
	}

	var resolvedUserID *uuid.UUID

	if parsedAuthMethod == AuthMethodRemote {
		if userID == nil {
			return nil, errorz.ValidationFailed{
				Field:   "user_id",
				Message: "cannot be empty for credential with auth method 'remote'",
			}
		}

		resolvedUserID = userID
	}

	parsedUsername, err := ParseUsername(username)
	if err != nil {
		return nil, err
	}

	parsedPassword, err := ParsePassword(password)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	now := time.Now().UTC()

	return &Credential{
		id:          id,
		createdAt:   now,
		updatedAt:   now,
		userID:      resolvedUserID,
		username:    parsedUsername,
		password:    parsedPassword,
		authMethod:  parsedAuthMethod,
		description: description,
	}, nil
}

func ParseUsername(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return "", errorz.ValidationFailed{Field: "username", Message: "cannot be empty"}
	}

	return trimmed, nil
}

func ParsePassword(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errorz.ValidationFailed{Field: "password", Message: "cannot be empty"}
	}

	return raw, nil
}

func ParseAuthMethod(raw string) (AuthMethod, error) {
	trimmed := strings.TrimSpace(raw)

	if AuthMethod(trimmed) == AuthMethodLocal || AuthMethod(trimmed) == AuthMethodRemote {
		return AuthMethod(trimmed), nil
	}

	return "", errorz.ValidationFailed{Field: "auth_method", Message: "must be a supported method (local | remote)"}
}

func UnmarshalCredential(id uuid.UUID, createdAt, updatedAt time.Time, userID *uuid.UUID, username, password string, authMethod AuthMethod, description *string, lastUsedAt *time.Time) *Credential {
	return &Credential{
		id:          id,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
		userID:      userID,
		username:    username,
		password:    password,
		authMethod:  authMethod,
		description: description,
		lastUsedAt:  lastUsedAt,
	}
}

type NetworkDevice struct {
	id                  uuid.UUID
	createdAt           time.Time
	updatedAt           time.Time
	ipAddress           netip.Addr
	hostname            *string
	serialNumber        *string
	productID           *string
	softwareVersion     *string
	sshActive           *bool
	netconfActive       *bool
	lastSyncStatus      SyncStatus
	lastSyncAttemptedAt *time.Time
}

type SyncStatus string

const (
	SyncStatusPending SyncStatus = "pending"
	SyncStatusSuccess SyncStatus = "success"
	SyncStatusFailure SyncStatus = "failure"
)

func (nd *NetworkDevice) ID() uuid.UUID {
	return nd.id
}
func (nd *NetworkDevice) CreatedAt() time.Time {
	return nd.createdAt
}

func (nd *NetworkDevice) UpdatedAt() time.Time {
	return nd.updatedAt
}

func (nd *NetworkDevice) IPAddress() netip.Addr {
	return nd.ipAddress
}

func (nd *NetworkDevice) Hostname() *string {
	return nd.hostname
}

func (nd *NetworkDevice) SerialNumber() *string {
	return nd.serialNumber
}

func (nd *NetworkDevice) ProductID() *string {
	return nd.productID
}

func (nd *NetworkDevice) SoftwareVersion() *string {
	return nd.softwareVersion
}

func (nd *NetworkDevice) SSHActive() *bool {
	return nd.sshActive
}

func (nd *NetworkDevice) NetconfActive() *bool {
	return nd.netconfActive
}

func (nd *NetworkDevice) LastSyncStatus() SyncStatus {
	return nd.lastSyncStatus
}

func (nd *NetworkDevice) LastSyncAttemptedAt() *time.Time {
	return nd.lastSyncAttemptedAt
}

type NetworkDeviceInventory struct {
	Hostname        string
	SerialNumber    string
	ProductID       string
	SoftwareVersion string
}

func (nd *NetworkDevice) ApplyReachableSyncFailure() {
	now := time.Now().UTC()

	nd.updatedAt = now
	nd.sshActive = new(true)
	nd.lastSyncStatus = SyncStatusFailure
	nd.lastSyncAttemptedAt = &now
}

func (nd *NetworkDevice) ApplyUnreachableSyncFailure() {
	now := time.Now().UTC()

	nd.updatedAt = now
	nd.sshActive = new(false)
	nd.netconfActive = new(false)
	nd.lastSyncStatus = SyncStatusFailure
	nd.lastSyncAttemptedAt = &now
}

func (nd *NetworkDevice) ApplySyncSuccess(results NetworkDeviceInventory) {
	now := time.Now().UTC()

	nd.updatedAt = now
	nd.hostname = new(results.Hostname)
	nd.serialNumber = new(results.SerialNumber)
	nd.productID = new(results.ProductID)
	nd.softwareVersion = new(results.SoftwareVersion)
	nd.sshActive = new(true)
	nd.lastSyncStatus = SyncStatusSuccess
	nd.lastSyncAttemptedAt = &now
}

func NewNetworkDevice(ipAddress string) (*NetworkDevice, error) {
	parsedIPAddress, err := ParseIPAddress(ipAddress)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	now := time.Now().UTC()

	return &NetworkDevice{
		id:             id,
		createdAt:      now,
		updatedAt:      now,
		ipAddress:      parsedIPAddress,
		lastSyncStatus: SyncStatusPending,
	}, nil
}

func ParseIPAddress(raw string) (netip.Addr, error) {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return netip.Addr{}, errorz.ValidationFailed{Field: "ip_address", Message: "cannot be empty"}
	}

	ipAddress, err := netip.ParseAddr(trimmed)
	if err != nil {
		return netip.Addr{}, errorz.ValidationFailed{Field: "ip_address", Message: "must be a valid IP address"}
	}

	// if !ipAddress.IsPrivate() {
	// 	return netip.Addr{}, errorz.ValidationFailed{Field: "ip_address", Message: "must be a private IP address"}
	// }

	return ipAddress, nil
}

func UnmarshalNetworkDevice(id uuid.UUID, createdAt, updatedAt time.Time, ipAddress netip.Addr, hostname *string, serialNumber, productID, softwareVersion *string, sshActive, netconfActive *bool, lastSyncStatus SyncStatus, lastSyncAttemptedAt *time.Time) *NetworkDevice {
	return &NetworkDevice{
		id:                  id,
		createdAt:           createdAt,
		updatedAt:           updatedAt,
		ipAddress:           ipAddress,
		hostname:            hostname,
		serialNumber:        serialNumber,
		productID:           productID,
		softwareVersion:     softwareVersion,
		sshActive:           sshActive,
		netconfActive:       netconfActive,
		lastSyncStatus:      lastSyncStatus,
		lastSyncAttemptedAt: lastSyncAttemptedAt,
	}
}
