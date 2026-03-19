package cisco

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/audit"
	"github.com/purpose-robot/planet-express/internal/errorz"
)

type Adapters struct {
	AuditLogRepository         audit.AuditLogRepository
	CredentialRepository       CredentialRepository
	NetworkDeviceRepository    NetworkDeviceRepository
	NetworkDeviceJobRepository NetworkDeviceJobRepository
}

type store interface {
	Transact(ctx context.Context, txFunc func(adapters Adapters) error) error
}

type CredentialRepository interface {
	Insert(ctx context.Context, credentials *Credential) error
	GetByID(ctx context.Context, id uuid.UUID) (*Credential, error)
}

type NetworkDeviceRepository interface {
	Insert(ctx context.Context, networkDevice *NetworkDevice) (bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*NetworkDevice, error)
	UpdateByID(ctx context.Context, id uuid.UUID, updateFn func(device *NetworkDevice) error) error
}

type NetworkDeviceJobRepository interface {
	CreateNetworkDeviceSyncJob(ctx context.Context, credentialID, networkDeviceID uuid.UUID) error
}

type NetworkDeviceInventoryCollector interface {
	Collect(ctx context.Context, ipAddress netip.Addr, credential *Credential) (NetworkDeviceInventory, error)
}

type Service struct {
	store  store
	config ServiceConfig
	logger *slog.Logger
	// auditLogRepository is used for writing audit logs outside of transactions using the outbox pattern.
	auditLogRepository audit.AuditLogRepository
	// userRepository and permissionRepository are used in case the SQL query should not be ran in transaction.
	credentialRepository    CredentialRepository
	networkDeviceRepository NetworkDeviceRepository
}

type ServiceConfig struct{}

func NewService(store store, config ServiceConfig, logger *slog.Logger, auditLogRepository audit.AuditLogRepository, credentialRepository CredentialRepository, networkDeviceRepository NetworkDeviceRepository) (*Service, error) {
	return &Service{
		store:                   store,
		config:                  config,
		logger:                  logger,
		auditLogRepository:      auditLogRepository,
		credentialRepository:    credentialRepository,
		networkDeviceRepository: networkDeviceRepository,
	}, nil
}

type AddCredential struct {
	UserID      *uuid.UUID
	Username    string
	Password    string
	AuthMethod  string
	Description *string
}

type AddCredentialResult struct {
	ID          uuid.UUID
	Username    string
	AuthMethod  AuthMethod
	Description *string
}

func (svc *Service) AddCredential(ctx context.Context, cmd AddCredential) (AddCredentialResult, error) {
	credential, err := NewCredential(cmd.UserID, cmd.Username, cmd.Password, cmd.AuthMethod, cmd.Description)
	if err != nil {
		return AddCredentialResult{}, mapDomainError(err)
	}

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		err := adapters.CredentialRepository.Insert(ctx, credential)
		if err != nil {
			return err
		}

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionCreated, cmd.UserID, credential.ID(), "credentials", map[string]string{"auth_method": string(credential.AuthMethod())}, nil)
	})
	if err != nil {
		return AddCredentialResult{}, mapDomainError(err)
	}

	return AddCredentialResult{
		ID:          credential.ID(),
		Username:    credential.Username(),
		AuthMethod:  credential.AuthMethod(),
		Description: credential.Description(),
	}, nil
}

type AddNetworkDevice struct {
	UserID       uuid.UUID
	IPAddress    string
	CredentialID uuid.UUID
}

type AddNetworkDeviceResult struct {
	ID             uuid.UUID
	IPAddress      netip.Addr
	LastSyncStatus SyncStatus
}

func (svc *Service) AddNetworkDevice(ctx context.Context, cmd AddNetworkDevice) (AddNetworkDeviceResult, error) {
	networkDevice, err := NewNetworkDevice(cmd.IPAddress)
	if err != nil {
		return AddNetworkDeviceResult{}, mapDomainError(err)
	}

	err = svc.store.Transact(ctx, func(adapters Adapters) error {
		credential, err := adapters.CredentialRepository.GetByID(ctx, cmd.CredentialID)
		if err != nil {
			if !errors.Is(err, errorz.ErrNotFound) {
				return err
			}

			return errorz.NewNotFound("the specified credential was not found", err)
		}

		if !credential.AccessibleBy(cmd.UserID) {
			return errorz.NewNotFound("the specified credential was not found", errorz.ErrRecordNotFound)
		}

		inserted, err := adapters.NetworkDeviceRepository.Insert(ctx, networkDevice)
		if err != nil {
			return err
		}

		if !inserted {
			return errorz.NewConflict("a network device with this IP address already exists", errorz.ErrConflict)
		}

		err = adapters.NetworkDeviceJobRepository.CreateNetworkDeviceSyncJob(ctx, credential.ID(), networkDevice.ID())
		if err != nil {
			return err
		}

		return audit.LogSuccess(ctx, adapters.AuditLogRepository, audit.ActionCreated, &cmd.UserID, networkDevice.ID(), "network_devices", map[string]string{"ip_address": networkDevice.IPAddress().String()}, nil)
	})
	if err != nil {
		return AddNetworkDeviceResult{}, mapDomainError(err)
	}

	return AddNetworkDeviceResult{
		ID:             networkDevice.ID(),
		IPAddress:      networkDevice.IPAddress(),
		LastSyncStatus: networkDevice.LastSyncStatus(),
	}, nil
}
