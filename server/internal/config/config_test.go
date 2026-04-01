package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)

	// Server defaults.
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 9090, cfg.Server.MetricsPort)
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "info", cfg.Server.LogLevel)
	assert.Equal(t, "json", cfg.Server.LogFormat)
	assert.Equal(t, 100, cfg.Server.RateLimitMax)
	assert.Equal(t, time.Minute, cfg.Server.RateLimitWindow)

	// Database defaults.
	assert.Contains(t, cfg.Database.URL, "postgres://")
	assert.Equal(t, 25, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)

	// Redis defaults.
	assert.Contains(t, cfg.Redis.URL, "redis://")
	assert.Equal(t, 3, cfg.Redis.MaxRetries)
	assert.Equal(t, 50, cfg.Redis.PoolSize)

	// Identity defaults.
	assert.Equal(t, "Ed25519", cfg.Identity.KeyType)
	assert.Equal(t, "aicp", cfg.Identity.DIDMethod)

	// Commerce defaults.
	assert.Equal(t, 5*time.Minute, cfg.Commerce.PaymentTokenTTL)
	assert.Equal(t, 15*time.Minute, cfg.Commerce.PaymentTokenMax)

	// Ledger defaults.
	assert.Equal(t, 10*time.Second, cfg.Ledger.BlockInterval)
	assert.Equal(t, 50, cfg.Ledger.MaxEntriesPerBlock)

	// Discovery defaults.
	assert.Equal(t, 5*time.Minute, cfg.Discovery.CacheTTL)
	assert.Equal(t, 50, cfg.Discovery.MaxSearchResults)
}

func TestServerConfig_Addr(t *testing.T) {
	cfg := ServerConfig{Host: "0.0.0.0", Port: 8080}
	assert.Equal(t, "0.0.0.0:8080", cfg.Addr())
}

func TestServerConfig_MetricsAddr(t *testing.T) {
	cfg := ServerConfig{Host: "0.0.0.0", MetricsPort: 9090}
	assert.Equal(t, "0.0.0.0:9090", cfg.MetricsAddr())
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:        8080,
			MetricsPort: 9090,
		},
		Commerce: CommerceConfig{
			PaymentTokenTTL: 5 * time.Minute,
			PaymentTokenMax: 15 * time.Minute,
		},
		Ledger: LedgerConfig{
			BlockInterval: 10 * time.Second,
		},
	}
	assert.NoError(t, validate(cfg))
}

func TestValidate_InvalidPort_Low(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 0, MetricsPort: 9090},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server port")
}

func TestValidate_InvalidPort_High(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 70000, MetricsPort: 9090},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server port")
}

func TestValidate_InvalidMetricsPort(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080, MetricsPort: 0},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metrics port")
}

func TestValidate_SamePort(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080, MetricsPort: 8080},
		Commerce: CommerceConfig{
			PaymentTokenTTL: 5 * time.Minute,
			PaymentTokenMax: 15 * time.Minute,
		},
		Ledger: LedgerConfig{BlockInterval: 10 * time.Second},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be different")
}

func TestValidate_PaymentTTLExceedsMax(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080, MetricsPort: 9090},
		Commerce: CommerceConfig{
			PaymentTokenTTL: 30 * time.Minute,
			PaymentTokenMax: 15 * time.Minute,
		},
		Ledger: LedgerConfig{BlockInterval: 10 * time.Second},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not exceed")
}

func TestValidate_BlockIntervalTooLow(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080, MetricsPort: 9090},
		Commerce: CommerceConfig{
			PaymentTokenTTL: 5 * time.Minute,
			PaymentTokenMax: 15 * time.Minute,
		},
		Ledger: LedgerConfig{BlockInterval: 500 * time.Millisecond},
	}
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block interval")
}

func TestLoad_OverrideFromEnv(t *testing.T) {
	t.Setenv("AICP_SERVER_PORT", "9999")
	t.Setenv("AICP_SERVER_METRICS_PORT", "9091")
	t.Setenv("AICP_SERVER_LOG_LEVEL", "debug")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.Server.Port)
	assert.Equal(t, 9091, cfg.Server.MetricsPort)
	assert.Equal(t, "debug", cfg.Server.LogLevel)
}

func TestLoad_CommerceOverrides(t *testing.T) {
	t.Setenv("AICP_COMMERCE_PAYMENT_TOKEN_TTL", "10m")
	t.Setenv("AICP_COMMERCE_PAYMENT_TOKEN_MAX", "30m")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 10*time.Minute, cfg.Commerce.PaymentTokenTTL)
	assert.Equal(t, 30*time.Minute, cfg.Commerce.PaymentTokenMax)
}

func TestLoad_LedgerOverrides(t *testing.T) {
	t.Setenv("AICP_LEDGER_BLOCK_INTERVAL", "30s")
	t.Setenv("AICP_LEDGER_MAX_ENTRIES_PER_BLOCK", "100")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.Ledger.BlockInterval)
	assert.Equal(t, 100, cfg.Ledger.MaxEntriesPerBlock)
}

func TestLoad_DiscoveryOverrides(t *testing.T) {
	t.Setenv("AICP_DISCOVERY_CACHE_TTL", "10m")
	t.Setenv("AICP_DISCOVERY_MAX_SEARCH_RESULTS", "100")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 10*time.Minute, cfg.Discovery.CacheTTL)
	assert.Equal(t, 100, cfg.Discovery.MaxSearchResults)
}
