package bessie

import (
	"context"
	"errors"

	"github.com/gofrs/uuid/v5"
	"github.com/purpose-robot/planet-express/internal/errorz"
	"github.com/riverqueue/river"
)

type SyncNetworkDeviceArgs struct {
	CredentialID    uuid.UUID `json:"credential_id"`
	NetworkDeviceID uuid.UUID `json:"network_device_id"`
}

func (SyncNetworkDeviceArgs) Kind() string {
	return "sync_network_device"
}

type SyncNetworkDeviceWorker struct {
	river.WorkerDefaults[SyncNetworkDeviceArgs]
	service   *Service
	collector NetworkDeviceInventoryCollector
}

func NewSyncNetworkDeviceWorker(service *Service, collector NetworkDeviceInventoryCollector) *SyncNetworkDeviceWorker {
	return &SyncNetworkDeviceWorker{
		service:   service,
		collector: collector,
	}
}

func (w *SyncNetworkDeviceWorker) Work(ctx context.Context, job *river.Job[SyncNetworkDeviceArgs]) error {
	credential, err := w.service.credentialRepository.GetByID(ctx, job.Args.CredentialID)
	if err != nil {
		if errors.Is(err, errorz.ErrNotFound) {
			return river.JobCancel(err)
		}

		return err
	}

	networkDevice, err := w.service.networkDeviceRepository.GetByID(ctx, job.Args.NetworkDeviceID)
	if err != nil {
		if errors.Is(err, errorz.ErrNotFound) {
			return river.JobCancel(err)
		}

		return err
	}

	inventory, err := w.collector.Collect(ctx, networkDevice.IPAddress(), credential)
	if err == nil {
		err = w.service.networkDeviceRepository.UpdateByID(ctx, networkDevice.ID(), func(networkDevice *NetworkDevice) error {
			networkDevice.ApplySyncSuccess(inventory)
			return nil
		})

		if !errors.Is(err, errorz.ErrNotFound) {
			return err
		}

		return river.JobCancel(err)
	}

	retryable := shouldRetrySync(err)

	updateErr := w.service.networkDeviceRepository.UpdateByID(ctx, networkDevice.ID(), func(networkDevice *NetworkDevice) error {
		if !retryable {
			networkDevice.ApplyReachableSyncFailure()
		} else {
			networkDevice.ApplyUnreachableSyncFailure()
		}

		return nil
	})
	if updateErr != nil {
		if !errors.Is(updateErr, errorz.ErrNotFound) {
			return updateErr
		}

		return river.JobCancel(updateErr)
	}

	if retryable {
		return err
	}

	if shouldCancelSync(err) {
		return river.JobCancel(err)
	}

	return err
}

func shouldRetrySync(err error) bool {
	return errors.Is(err, ErrDiscovererConnectionFailed) || errors.Is(err, ErrDiscovererConnectionLost)
}

func shouldCancelSync(err error) bool {
	return errors.Is(err, ErrDiscovererInputFailed) || errors.Is(err, ErrDiscovererUnexpectedOutput) || errors.Is(err, ErrDiscovererInvalidOptions)
}
