package config

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/purpose-robot/planet-express/internal/auth"
	"github.com/purpose-robot/planet-express/internal/cisco"
	"github.com/purpose-robot/planet-express/internal/health"
	"github.com/purpose-robot/planet-express/internal/krypto"
)

const maxStringLength = 512

type Config struct {
	DB      dbConfig
	HTTP    httpConfig
	Email   emailConfig
	Service serviceConfig
}

type dbConfig struct {
	Kind           string
	Postgres       postgresConfig
	BlindIndex     krypto.Key
	EncryptionKeys []krypto.Key
}

type httpConfig struct {
	Port            int
	Limiter         limiterConfig
	IdleTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type emailConfig struct {
	Kind string
	SMTP smtpConfig
}

type serviceConfig struct {
	Auth  authServiceConfig
	Email emailServiceConfig
	River riverConfig
	Cisco ciscoServiceConfig
}

type postgresConfig struct {
	Name     string
	Port     int
	Host     string
	Username string
	Password string
}

type limiterConfig struct {
	RPS     int
	Burst   int
	Enabled bool
}

type smtpConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

type authServiceConfig struct {
	DefaultPermissions   []auth.Permission
	ActivationExpiry     time.Duration
	PasswordResetExpiry  time.Duration
	AuthenticationExpiry time.Duration
}

type emailServiceConfig struct {
	From string
}

type riverConfig struct {
	MaxWorkers      int
	JobTimeout      time.Duration
	ShutdownTimeout time.Duration
}

type ciscoServiceConfig struct {
	SSH ciscoSSHConfig
}

type ciscoSSHConfig struct {
	Port             int
	Timeout          time.Duration
	TCPDialTimeout   time.Duration
	OperationTimeout time.Duration
}

func defaultConfig() Config {
	return Config{
		HTTP: httpConfig{
			Port: 11880,
			Limiter: limiterConfig{
				RPS:     10,
				Burst:   100,
				Enabled: true,
			},
			IdleTimeout:     time.Minute,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Service: serviceConfig{
			Auth: authServiceConfig{
				DefaultPermissions: []auth.Permission{
					cisco.PermissionCredentialsRead,
					cisco.PermissionCredentialsWrite,
					cisco.PermissionNetworkDevicesRead,
					cisco.PermissionNetworkDevicesWrite,
					health.PermissionRead,
				},
				ActivationExpiry:     24 * time.Hour,
				PasswordResetExpiry:  30 * time.Minute,
				AuthenticationExpiry: 12 * time.Hour,
			},
			Email: emailServiceConfig{
				From: "noreply@planet-express.ai",
			},
			River: riverConfig{
				MaxWorkers:      10,
				JobTimeout:      time.Minute,
				ShutdownTimeout: 10 * time.Second,
			},
			Cisco: ciscoServiceConfig{
				SSH: ciscoSSHConfig{
					Port:             22,
					Timeout:          10 * time.Second,
					TCPDialTimeout:   10 * time.Second,
					OperationTimeout: 10 * time.Second,
				},
			},
		},
	}
}

type envVariable struct {
	req     bool
	mapFunc func(v string, c *Config) error
}

var envMap = map[string]envVariable{
	"DB_KIND": {
		req: true,
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.DB.Kind, 1, maxStringLength)
		},
	},

	"DB_POSTGRES_NAME": {
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.DB.Postgres.Name, 1, maxStringLength)
		},
	},

	"DB_POSTGRES_PORT": {
		mapFunc: func(v string, c *Config) error {
			return confInt(v, &c.DB.Postgres.Port, 1, 65535)
		},
	},

	"DB_POSTGRES_HOST": {
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.DB.Postgres.Host, 1, maxStringLength)
		},
	},

	"DB_POSTGRES_USERNAME": {
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.DB.Postgres.Username, 1, maxStringLength)
		},
	},

	"DB_POSTGRES_PASSWORD": {
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.DB.Postgres.Password, 1, maxStringLength)
		},
	},

	"DB_BLIND_INDEX": {
		req: true,
		mapFunc: func(v string, c *Config) error {
			return confCryptoKey(v, &c.DB.BlindIndex)
		},
	},

	"DB_ENCRYPTION_KEYS": {
		req: true,
		mapFunc: func(v string, c *Config) error {
			return confSliceOf(v, &c.DB.EncryptionKeys, krypto.ParseKey, 1, 8)
		},
	},

	"HTTP_PORT": {
		mapFunc: func(v string, c *Config) error {
			return confInt(v, &c.HTTP.Port, 1, 65535)
		},
	},

	"HTTP_LIMITER_RPS": {
		mapFunc: func(v string, c *Config) error {
			return confInt(v, &c.HTTP.Limiter.RPS, 1, 100)
		},
	},

	"HTTP_LIMITER_BURST": {
		mapFunc: func(v string, c *Config) error {
			return confInt(v, &c.HTTP.Limiter.Burst, 10, 1000)
		},
	},

	"HTTP_LIMITER_ENABLED": {
		mapFunc: func(v string, c *Config) error {
			return confBool(v, &c.HTTP.Limiter.Enabled)
		},
	},

	"HTTP_IDLE_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.HTTP.IdleTimeout, 5*time.Second, 30*time.Second)
		},
	},

	"HTTP_READ_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.HTTP.ReadTimeout, 5*time.Second, 30*time.Second)
		},
	},

	"HTTP_WRITE_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.HTTP.WriteTimeout, 5*time.Second, 30*time.Second)
		},
	},

	"HTTP_SHUTDOWN_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.HTTP.ShutdownTimeout, 10*time.Second, 30*time.Second)
		},
	},

	"EMAIL_KIND": {
		req: true,
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.Email.Kind, 1, maxStringLength)
		},
	},

	"EMAIL_SMTP_PORT": {
		mapFunc: func(v string, c *Config) error {
			return confInt(v, &c.Email.SMTP.Port, 1, 65535)
		},
	},

	"EMAIL_SMTP_HOST": {
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.Email.SMTP.Host, 1, maxStringLength)
		},
	},

	"EMAIL_SMTP_USERNAME": {
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.Email.SMTP.Username, 1, maxStringLength)
		},
	},

	"EMAIL_SMTP_PASSWORD": {
		mapFunc: func(v string, c *Config) error {
			return confString(v, &c.Email.SMTP.Password, 1, maxStringLength)
		},
	},

	"SERVICE_AUTH_DEFAULT_PERMISSIONS": {
		mapFunc: func(v string, c *Config) error {
			return confSliceOf(v, &c.Service.Auth.DefaultPermissions, auth.ParsePermission, 1, 64)
		},
	},

	"SERVICE_AUTH_ACTIVATION_EXPIRY": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.Auth.ActivationExpiry, 8*time.Hour, 72*time.Hour)
		},
	},

	"SERVICE_AUTH_PASSWORD_RESET_EXPIRY": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.Auth.PasswordResetExpiry, 5*time.Minute, time.Hour)
		},
	},

	"SERVICE_AUTH_AUTHENTICATION_EXPIRY": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.Auth.AuthenticationExpiry, time.Hour, 24*time.Hour)
		},
	},

	"SERVICE_EMAIL_FROM": {
		mapFunc: func(v string, c *Config) error {
			return confEmail(v, &c.Service.Email.From)
		},
	},

	"SERVICE_RIVER_MAX_WORKERS": {
		mapFunc: func(v string, c *Config) error {
			return confInt(v, &c.Service.River.MaxWorkers, 3, 10)
		},
	},

	"SERVICE_RIVER_JOB_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.River.JobTimeout, time.Minute, 10*time.Minute)
		},
	},

	"SERVICE_RIVER_SHUTDOWN_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.River.ShutdownTimeout, 10*time.Second, 30*time.Second)
		},
	},

	"SERVICE_CISCO_SSH_PORT": {
		mapFunc: func(v string, c *Config) error {
			return confInt(v, &c.Service.Cisco.SSH.Port, 1, 65535)
		},
	},

	"SERVICE_CISCO_SSH_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.Cisco.SSH.Timeout, time.Second, 30*time.Second)
		},
	},

	"SERVICE_CISCO_SSH_TCP_DIAL_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.Cisco.SSH.TCPDialTimeout, time.Second, time.Minute)
		},
	},

	"SERVICE_CISCO_SSH_OPERATION_TIMEOUT": {
		mapFunc: func(v string, c *Config) error {
			return confDuration(v, &c.Service.Cisco.SSH.OperationTimeout, time.Second, 2*time.Minute)
		},
	},
}

func New() (Config, error) {
	config := defaultConfig()

	var errMap error

	for key, value := range envMap {
		found, ok := os.LookupEnv(key)
		if !ok {
			if value.req {
				errMap = errors.Join(errMap, fmt.Errorf("missing required environment variable %s", key))
			}

			continue
		}

		if err := value.mapFunc(found, &config); err != nil {
			errMap = errors.Join(errMap, fmt.Errorf("failed to load environment variable %s: %w", key, err))
		}
	}

	return config, errMap
}

func confInt(v string, out *int, min, max int) error {
	i, err := strconv.Atoi(v)
	if err != nil {
		return err
	}

	if i < min || i > max {
		return fmt.Errorf("integer %d not in range [%d, %d] (inclusive)", i, min, max)
	}

	*out = i
	return nil
}

func confBool(v string, out *bool) error {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return err
	}

	*out = b
	return nil
}

func confEmail(v string, out *string) error {
	trimmed := strings.TrimSpace(v)

	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return err
	}

	if addr.Address != trimmed {
		return fmt.Errorf("email address contains names and/or comments")
	}

	*out = addr.Address
	return nil
}

func confDuration(v string, out *time.Duration, min, max time.Duration) error {
	d, err := time.ParseDuration(v)
	if err != nil {
		return err
	}

	if d < min || d > max {
		return fmt.Errorf("duration %s not in range [%s, %s] (inclusive)", d, min, max)
	}

	*out = d
	return nil
}

func confCryptoKey(v string, out *krypto.Key) error {
	k, err := krypto.ParseKey(v)
	if err != nil {
		return err
	}

	*out = k
	return nil
}

func confString(v string, out *string, minLen, maxLen int) error {
	trimmed := strings.TrimSpace(v)

	if len(trimmed) < minLen || len(trimmed) > maxLen {
		return fmt.Errorf("string length %d not in range [%d, %d] (inclusive)", len(trimmed), minLen, maxLen)
	}

	*out = trimmed
	return nil
}

func confSliceOf[T any](v string, out *[]T, elementFunc func(string) (T, error), minLen, maxLen int) error {
	parts := strings.Split(v, ",")
	parsedValues := make([]T, 0, len(parts))

	for i, element := range parts {
		parsed, err := elementFunc(strings.TrimSpace(element))
		if err != nil {
			return fmt.Errorf("failed to parse element %d: %w", i, err)
		}

		parsedValues = append(parsedValues, parsed)
	}

	if len(parsedValues) < minLen || len(parsedValues) > maxLen {
		return fmt.Errorf("slice length %d not in range [%d, %d] (inclusive)", len(parsedValues), minLen, maxLen)
	}

	*out = parsedValues
	return nil
}
