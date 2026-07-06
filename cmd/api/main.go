package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/purpose-robot/planet-express/internal/audit"
	auditpostgres "github.com/purpose-robot/planet-express/internal/audit/postgres"
	auditriverx "github.com/purpose-robot/planet-express/internal/audit/riverx"
	"github.com/purpose-robot/planet-express/internal/auth"
	authpostgres "github.com/purpose-robot/planet-express/internal/auth/postgres"
	"github.com/purpose-robot/planet-express/internal/cisco"
	ciscopostgres "github.com/purpose-robot/planet-express/internal/cisco/postgres"
	ciscossh "github.com/purpose-robot/planet-express/internal/cisco/ssh"
	"github.com/purpose-robot/planet-express/internal/config"
	"github.com/purpose-robot/planet-express/internal/email"
	"github.com/purpose-robot/planet-express/internal/email/smtp"
	"github.com/purpose-robot/planet-express/internal/email/views"
	"github.com/purpose-robot/planet-express/internal/health"
	"github.com/purpose-robot/planet-express/internal/krypto"
	"github.com/purpose-robot/planet-express/internal/version"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func main() {
	ctx := context.Background()

	err := run(ctx, os.Stdout, os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

type application struct {
	logger        *slog.Logger
	config        config.Config
	riverClient   *river.Client[pgx.Tx]
	authService   *auth.Service
	ciscoService  *cisco.Service
	healthService *health.Service
}

func run(ctx context.Context, w io.Writer, _ []string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	logger := newLogger(w)

	config, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to fetch config from environment: \n%w", err)
	}

	dbPool, err := newDBPool(ctx, config)
	if err != nil {
		return err
	}
	defer dbPool.Close()

	auditRepository := auditpostgres.NewAuditLogRepository(dbPool)

	encryptor, err := krypto.NewEncryptor(config.DB.EncryptionKeys)
	if err != nil {
		return fmt.Errorf("failed to create encryptor for auth service: %w", err)
	}

	workers := river.NewWorkers()

	riverClient, err := newRiverClient(dbPool, workers, config, logger, auditRepository)
	if err != nil {
		return err
	}

	emailer, err := newEmailService(config)
	if err != nil {
		return err
	}

	authService, err := newAuthService(dbPool, encryptor, riverClient, config, logger, auditRepository)
	if err != nil {
		return err
	}

	ciscoService, err := newCiscoService(dbPool, encryptor, riverClient, config, logger, auditRepository)
	if err != nil {
		return err
	}

	collector := ciscossh.NewInventoryCollector(ciscossh.Config{
		Port:             config.Service.Cisco.SSH.Port,
		CloseTimeout:     config.Service.Cisco.SSH.CloseTimeout,
		TCPDialTimeout:   config.Service.Cisco.SSH.TCPDialTimeout,
		OperationTimeout: config.Service.Cisco.SSH.OperationTimeout,
	})

	river.AddWorker(workers, auth.NewSendActivationEmailWorker(emailer))
	river.AddWorker(workers, auth.NewSendPasswordResetEmailWorker(emailer))
	river.AddWorker(workers, auditriverx.NewAuditLogWorker(auditRepository))
	river.AddWorker(workers, cisco.NewSyncNetworkDeviceWorker(ciscoService, collector))

	app := &application{
		logger:        logger,
		config:        config,
		riverClient:   riverClient,
		authService:   authService,
		ciscoService:  ciscoService,
		healthService: health.NewService(logger, version.Get()),
	}

	err = riverClient.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start river client: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.Service.River.ShutdownTimeout)
		defer cancel()

		_ = riverClient.Stop(shutdownCtx)
	}()

	return app.listenAndServe(ctx)
}

func newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

func newDBPool(ctx context.Context, config config.Config) (*pgxpool.Pool, error) {
	switch config.DB.Kind {
	case "postgres":
		connString, err := pgxpool.ParseConfig("")
		if err != nil {
			return nil, fmt.Errorf("failed to creater default pool configuration: %w", err)
		}

		connString.ConnConfig.Host = config.DB.Postgres.Host
		connString.ConnConfig.Port = uint16(config.DB.Postgres.Port)
		connString.ConnConfig.Database = config.DB.Postgres.Name
		connString.ConnConfig.User = config.DB.Postgres.Username
		connString.ConnConfig.Password = config.DB.Postgres.Password

		dbPool, err := pgxpool.NewWithConfig(ctx, connString)
		if err != nil {
			return nil, fmt.Errorf("failed to create connection pool: %w", err)
		}

		return dbPool, nil

	default:
		return nil, fmt.Errorf("unsupported database kind: %s", config.DB.Kind)
	}
}

func newRiverClient(dbPool *pgxpool.Pool, workers *river.Workers, config config.Config, logger *slog.Logger, auditRepository audit.AuditLogRepository) (*river.Client[pgx.Tx], error) {
	riverLogger := logger.With(
		slog.String("component", "river"),
		slog.String("subsystem", "worker"),
	)

	riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
		Logger: riverLogger,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {
				MaxWorkers: config.Service.River.MaxWorkers,
			},
		},
		Workers:      workers,
		JobTimeout:   config.Service.River.JobTimeout,
		ErrorHandler: auditriverx.NewErrorHandler(riverLogger, auditRepository),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create river client using the pgx driver: %w", err)
	}

	return riverClient, nil
}

func newEmailService(config config.Config) (*email.Service, error) {
	sender, err := newEmailSender(config)
	if err != nil {
		return nil, err
	}

	renderer, err := newEmailRenderer()
	if err != nil {
		return nil, err
	}

	return email.NewService(email.ServiceConfig{From: config.Service.Email.From}, sender, renderer), nil
}

func newEmailSender(config config.Config) (email.Sender, error) {
	switch config.Email.Kind {
	case "smtp":
		sender, err := smtp.NewSender(
			config.Email.SMTP.Host,
			config.Email.SMTP.Port,
			config.Email.SMTP.Username,
			config.Email.SMTP.Password,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create email sender: %w", err)
		}

		return sender, nil

	default:
		return nil, fmt.Errorf("unsupported email kind: %s", config.Email.Kind)
	}
}

func newEmailRenderer() (email.Renderer, error) {
	emailFS, err := fs.Sub(email.FS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to parse email subtree folder: %w", err)
	}

	renderer, err := views.NewRenderer(emailFS)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer for emailer: %w", err)
	}

	return renderer, nil
}

func newAuthService(dbPool *pgxpool.Pool, encryptor *krypto.Encryptor, riverClient *river.Client[pgx.Tx], config config.Config, logger *slog.Logger, auditRepository audit.AuditLogRepository) (*auth.Service, error) {
	service, err := auth.NewService(
		authpostgres.NewStore(dbPool, encryptor, config.DB.BlindIndex, riverClient),
		auth.ServiceConfig{
			DefaultPermissions:   config.Service.Auth.DefaultPermissions,
			ActivationExpiry:     config.Service.Auth.ActivationExpiry,
			PasswordResetExpiry:  config.Service.Auth.PasswordResetExpiry,
			AuthenticationExpiry: config.Service.Auth.AuthenticationExpiry,
		},
		logger.With(
			slog.String("component", "app"),
			slog.String("subsystem", "auth"),
		),
		auditRepository,
		authpostgres.NewUserRepository(dbPool, encryptor, config.DB.BlindIndex),
		authpostgres.NewPermissionRepository(dbPool),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create and initialize %q subsystem: %w", "auth", err)
	}

	return service, nil
}

func newCiscoService(dbPool *pgxpool.Pool, encryptor *krypto.Encryptor, riverClient *river.Client[pgx.Tx], config config.Config, logger *slog.Logger, auditRepository audit.AuditLogRepository) (*cisco.Service, error) {
	service, err := cisco.NewService(
		ciscopostgres.NewStore(dbPool, encryptor, config.DB.BlindIndex, riverClient),
		cisco.ServiceConfig{},
		logger.With(
			slog.String("component", "app"),
			slog.String("subsystem", "cisco"),
		),
		auditRepository,
		ciscopostgres.NewCredentialRepository(dbPool, encryptor, config.DB.BlindIndex),
		ciscopostgres.NewNetworkDeviceRepository(dbPool),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create and initialize %q subsystem: %w", "cisco", err)
	}

	return service, nil
}
