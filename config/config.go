package config

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/ilyakaznacheev/cleanenv"
)

type (
	Config struct {
		App        `json:"app"     toml:"app"`
		Blockchain `json:"blockchain" toml:"blockchain"`
		HTTP       `json:"http"    toml:"http"`
		DB         `json:"db"      toml:"db"`
		Log        `json:"logger"  toml:"logger"`
		Tracing    `json:"tracing" toml:"tracing"`
		AML        `json:"aml"     toml:"aml"`
		Workers    `json:"workers" toml:"workers"`
	}

	App struct {
		Name        string `json:"name"        toml:"name"        env:"APP_NAME"`
		Environment string `json:"environment" toml:"environment" env:"ENV_NAME" env-default:"dev"`
		Debug       bool   `json:"debug"       toml:"debug"       env:"DEBUG"    env-default:"false"`
	}

	HTTP struct {
		Port string ` json:"port" toml:"port" env:"HTTP_PORT"`
	}

	DB struct {
		DatabaseURL       string `json:"database_url"     toml:"database_url"     env:"DATABASE_URL"`
		PoolMax           int32  `json:"pool_max" toml:"pool_max" env:"PG_POOL_MAX" env-required:"true"`
		ConnectTimeout    int    `json:"connect_timeout" toml:"connect_timeout" env:"PG_POOL_CONN_TIMEOUT" env-default:"5"`
		HealthCheckPeriod int    `json:"health_check_period" toml:"health_check_period" env:"PG_POOL_HEALTHCHECK" env-default:"1"`
	}

	Blockchain struct {
		Debug                 bool   `json:"blockchain_debug" toml:"blockchain_debug" env:"BLOCKCHAIN_DEBUG_MODE" env-default:"false"`
		RPCURL                string `json:"rpc_url" toml:"rpc_url" env:"RPC_URL" env-default:"https://bsc-dataseed.binance.org/"`
		WalletSeed            string `json:"wallet_seed" toml:"wallet_seed" env:"WALLET_SEED" env-default:"your secure seed phrase here"`
		RequiredConfirmations uint64 `json:"required_confirmations" toml:"required_confirmations" env:"REQUIRED_CONFIRMATIONS" env-default:"3"`
	}

	AML struct {
		// Chainalysis API configuration
		ChainalysisAPIKey string `json:"chainalysis_api_key" toml:"chainalysis_api_key" env:"CHAINALYSIS_API_KEY" env-default:""`
		ChainalysisAPIURL string `json:"chainalysis_api_url" toml:"chainalysis_api_url" env:"CHAINALYSIS_API_URL" env-default:"https://api.chainalysis.com/v1"`

		// Elliptic/TRM Labs API configuration
		EllipticAPIKey string `json:"elliptic_api_key" toml:"elliptic_api_key" env:"ELLIPTIC_API_KEY" env-default:""`
		EllipticAPIURL string `json:"elliptic_api_url" toml:"elliptic_api_url" env:"ELLIPTIC_API_URL" env-default:"https://api.trmlabs.com/v1"`

		// AMLBot API configuration
		AMLBotAPIKey string `json:"amlbot_api_key" toml:"amlbot_api_key" env:"AMLBOT_API_KEY" env-default:""`
		AMLBotAPIURL string `json:"amlbot_api_url" toml:"amlbot_api_url" env:"AMLBOT_API_URL" env-default:"https://api.amlbot.com/v1"`

		// Local AML checks configuration
		TransactionThreshold string `json:"transaction_threshold" toml:"transaction_threshold" env:"AML_TRANSACTION_THRESHOLD" env-default:"5000.0"`
	}

	Log struct {
		Level slog.Level `json:"level" toml:"level" env:"LOG_LEVEL"`
	}

	Tracing struct {
		URL string ` json:"url" toml:"url" env:"TRACING_URL"`
	}

	Workers struct {
		OrderExpiration      int `json:"order_expiration" toml:"order_expiration" env:"ORDER_EXPIRATION" env-default:"180"`                 // Default 180 minutes (3 hours)
		OrderCleanupInterval int `json:"order_cleanup_interval" toml:"order_cleanup_interval" env:"ORDER_CLEANUP_INTERVAL" env-default:"5"` // Default 5 minutes
	}
)

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	_, b, _, _ := runtime.Caller(0)
	basePath := filepath.Dir(b)

	configTomlPath := filepath.Join(basePath, "config.toml")
	err := cleanenv.ReadConfig(configTomlPath, cfg)
	if err != nil {
		configJsonPath := filepath.Join(basePath, "config.json")
		err = cleanenv.ReadConfig(configJsonPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
	}

	err = cleanenv.ReadEnv(cfg)
	if err != nil {
		return nil, fmt.Errorf("env read error: %w", err)
	}

	return cfg, nil
}
