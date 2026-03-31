// Package config provides validated server configuration loaded from
// environment variables with sensible defaults for production deployment.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds the complete server configuration. All fields are populated
// from environment variables prefixed with AICP_. Struct tags define the
// env var name, default value, and whether the field is required.
type Config struct {
	Server    ServerConfig    `envPrefix:"AICP_SERVER_"`
	Database  DatabaseConfig  `envPrefix:"AICP_DATABASE_"`
	Redis     RedisConfig     `envPrefix:"AICP_REDIS_"`
	Identity  IdentityConfig  `envPrefix:"AICP_IDENTITY_"`
	Commerce  CommerceConfig  `envPrefix:"AICP_COMMERCE_"`
	Ledger    LedgerConfig    `envPrefix:"AICP_LEDGER_"`
	Discovery DiscoveryConfig `envPrefix:"AICP_DISCOVERY_"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string        `env:"HOST"              envDefault:"0.0.0.0"`
	Port            int           `env:"PORT"              envDefault:"8080"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT"      envDefault:"10s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT"     envDefault:"30s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT"      envDefault:"120s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT"  envDefault:"15s"`
	MetricsPort     int           `env:"METRICS_PORT"      envDefault:"9090"`
	LogLevel        string        `env:"LOG_LEVEL"         envDefault:"info"`
	LogFormat       string        `env:"LOG_FORMAT"        envDefault:"json"`
}

// Addr returns the server listen address.
func (c ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// MetricsAddr returns the metrics server listen address.
func (c ServerConfig) MetricsAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.MetricsPort)
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL             string        `env:"URL"               envDefault:"postgres://aicp:aicp@localhost:5432/aicp?sslmode=disable"`
	MaxOpenConns    int           `env:"MAX_OPEN_CONNS"    envDefault:"25"`
	MaxIdleConns    int           `env:"MAX_IDLE_CONNS"    envDefault:"10"`
	ConnMaxLifetime time.Duration `env:"CONN_MAX_LIFETIME" envDefault:"30m"`
	ConnMaxIdleTime time.Duration `env:"CONN_MAX_IDLE"     envDefault:"5m"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL          string        `env:"URL"            envDefault:"redis://localhost:6379/0"`
	MaxRetries   int           `env:"MAX_RETRIES"    envDefault:"3"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT"   envDefault:"3s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT"  envDefault:"3s"`
	PoolSize     int           `env:"POOL_SIZE"      envDefault:"50"`
}

// IdentityConfig holds settings for the identity layer.
type IdentityConfig struct {
	KeyType              string        `env:"KEY_TYPE"                envDefault:"Ed25519"`
	DIDMethod            string        `env:"DID_METHOD"              envDefault:"aicp"`
	CredentialTTL        time.Duration `env:"CREDENTIAL_TTL"          envDefault:"8760h"` // 1 year
	KeyRotationGracePeriod time.Duration `env:"KEY_ROTATION_GRACE"    envDefault:"24h"`
}

// CommerceConfig holds settings for the commerce layer.
type CommerceConfig struct {
	PaymentTokenTTL  time.Duration `env:"PAYMENT_TOKEN_TTL"  envDefault:"5m"`
	PaymentTokenMax  time.Duration `env:"PAYMENT_TOKEN_MAX"  envDefault:"15m"`
	EnabledAdapters  []string      `env:"ENABLED_ADAPTERS"   envDefault:"stripe,paypal,usdc" envSeparator:","`
	Stripe           StripeConfig  `envPrefix:"STRIPE_"`
	PayPal           PayPalConfig  `envPrefix:"PAYPAL_"`
	USDC             USDCConfig    `envPrefix:"USDC_"`
}

// StripeConfig holds Stripe-specific settings.
type StripeConfig struct {
	SecretKey      string `env:"SECRET_KEY"      envDefault:""`
	WebhookSecret  string `env:"WEBHOOK_SECRET"  envDefault:""`
}

// PayPalConfig holds PayPal-specific settings.
type PayPalConfig struct {
	ClientID     string `env:"CLIENT_ID"     envDefault:""`
	ClientSecret string `env:"CLIENT_SECRET" envDefault:""`
	Sandbox      bool   `env:"SANDBOX"       envDefault:"true"`
}

// USDCConfig holds USDC/Circle-specific settings.
type USDCConfig struct {
	APIKey    string `env:"API_KEY"    envDefault:""`
	EntityID  string `env:"ENTITY_ID"  envDefault:""`
	Sandbox   bool   `env:"SANDBOX"    envDefault:"true"`
}

// LedgerConfig holds consortium ledger settings.
type LedgerConfig struct {
	ValidatorDID       string        `env:"VALIDATOR_DID"        envDefault:""`
	ValidatorKeyPath   string        `env:"VALIDATOR_KEY_PATH"   envDefault:""`
	BlockInterval      time.Duration `env:"BLOCK_INTERVAL"       envDefault:"10s"`
	MaxEntriesPerBlock int           `env:"MAX_ENTRIES_PER_BLOCK" envDefault:"50"`
	ConsensusTimeout   time.Duration `env:"CONSENSUS_TIMEOUT"    envDefault:"5s"`
	PeerAddresses      []string      `env:"PEER_ADDRESSES"       envDefault:"" envSeparator:","`
	GRPCPort           int           `env:"GRPC_PORT"            envDefault:"9091"`
}

// DiscoveryConfig holds ANS discovery settings.
type DiscoveryConfig struct {
	CacheTTL          time.Duration `env:"CACHE_TTL"           envDefault:"5m"`
	MaxSearchResults  int           `env:"MAX_SEARCH_RESULTS"  envDefault:"50"`
	WellKnownPath     string        `env:"WELL_KNOWN_PATH"     envDefault:"/.well-known/aicp.json"`
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config from environment: %w", err)
	}
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}
	return cfg, nil
}

// validate performs cross-field validation that cannot be expressed via struct tags.
func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", cfg.Server.Port)
	}
	if cfg.Server.MetricsPort < 1 || cfg.Server.MetricsPort > 65535 {
		return fmt.Errorf("metrics port must be between 1 and 65535, got %d", cfg.Server.MetricsPort)
	}
	if cfg.Server.Port == cfg.Server.MetricsPort {
		return fmt.Errorf("server port and metrics port must be different")
	}
	if cfg.Commerce.PaymentTokenTTL > cfg.Commerce.PaymentTokenMax {
		return fmt.Errorf("payment token TTL (%s) must not exceed max TTL (%s)",
			cfg.Commerce.PaymentTokenTTL, cfg.Commerce.PaymentTokenMax)
	}
	if cfg.Ledger.BlockInterval < time.Second {
		return fmt.Errorf("block interval must be at least 1s, got %s", cfg.Ledger.BlockInterval)
	}
	return nil
}
