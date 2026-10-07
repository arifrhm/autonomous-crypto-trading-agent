package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Binance  BinanceConfig
	Database DatabaseConfig
	Redis    RedisConfig
}

type AppConfig struct {
	Env  string
	Port int
}

type BinanceConfig struct {
	WSURL   string
	Symbols []string
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// Load reads configuration from environment variables and optional .env file.
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig() // ignore error if .env doesn't exist, fallback to OS env

	v.AutomaticEnv()

	// Defaults
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("BINANCE_WS_URL", "wss://stream.binance.com:9443/stream")
	v.SetDefault("TRADING_SYMBOLS", "btcusdt,ethusdt")

	v.SetDefault("DATABASE_HOST", "localhost")
	v.SetDefault("DATABASE_PORT", 5432)
	v.SetDefault("DATABASE_USER", "postgres")
	v.SetDefault("DATABASE_PASSWORD", "postgres")
	v.SetDefault("DATABASE_NAME", "crypto_agent")
	v.SetDefault("DATABASE_SSLMODE", "disable")

	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)

	symbolsRaw := v.GetString("TRADING_SYMBOLS")
	symbols := strings.Split(symbolsRaw, ",")
	for i := range symbols {
		symbols[i] = strings.TrimSpace(symbols[i])
	}

	cfg := &Config{
		App: AppConfig{
			Env:  v.GetString("APP_ENV"),
			Port: v.GetInt("APP_PORT"),
		},
		Binance: BinanceConfig{
			WSURL:   v.GetString("BINANCE_WS_URL"),
			Symbols: symbols,
		},
		Database: DatabaseConfig{
			Host:            v.GetString("DATABASE_HOST"),
			Port:            v.GetInt("DATABASE_PORT"),
			User:            v.GetString("DATABASE_USER"),
			Password:        v.GetString("DATABASE_PASSWORD"),
			Name:            v.GetString("DATABASE_NAME"),
			SSLMode:         v.GetString("DATABASE_SSLMODE"),
			MaxConns:        25,
			MinConns:        5,
			MaxConnLifetime: 1 * time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
		},
		Redis: RedisConfig{
			Host:     v.GetString("REDIS_HOST"),
			Port:     v.GetInt("REDIS_PORT"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
		},
	}

	if len(cfg.Binance.Symbols) == 0 || cfg.Binance.Symbols[0] == "" {
		return nil, fmt.Errorf("TRADING_SYMBOLS cannot be empty")
	}

	return cfg, nil
}
