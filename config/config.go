package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
	"path/filepath"
	"runtime"
)

type (
	Config struct {
		App        `json:"app"     toml:"app"`
		Blockchain `json:"blockchain" toml:"blockchain"`
		HTTP       `json:"http"    toml:"http"`
		DB         `json:"db"      toml:"db"`
		Log        `json:"logger"  toml:"logger"`
		Tracing    `json:"tracing" toml:"tracing"`
	}

	Blockchain struct {
		RPCURL                string `json:"rpc_url" toml:"rpc_url" env-default:"https://bsc-dataseed.binance.org/"`
		WalletSeed            string `json:"wallet_seed" toml:"wallet_seed" env-default:"your secure seed phrase here"`
		RequiredConfirmations uint64 `json:"required_confirmations" toml:"required_confirmations" env-default:"3"`
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

	Log struct {
		Level slog.Level `json:"level" toml:"level" env:"LOG_LEVEL"`
	}

	Tracing struct {
		URL string ` json:"url" toml:"url" env:"TRACING_URL"`
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
