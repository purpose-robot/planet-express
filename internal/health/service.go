package health

import (
	"log/slog"
	"time"
)

type Service struct {
	logger    *slog.Logger
	version   string
	startedAt time.Time
}

func NewService(logger *slog.Logger, version string) *Service {
	return &Service{
		logger:    logger,
		version:   version,
		startedAt: time.Now().UTC(),
	}
}

type CheckHealthResponse struct {
	Status  string
	Uptime  string
	Version string
}

func (s *Service) CheckHealth() (CheckHealthResponse, error) {
	return CheckHealthResponse{
		Status:  "ok",
		Uptime:  time.Since(s.startedAt).Round(time.Second).String(),
		Version: s.version,
	}, nil
}
