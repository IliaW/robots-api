package config

import (
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env                string            `mapstructure:"env"`
	LogLevel           string            `mapstructure:"log_level"`
	LogType            string            `mapstructure:"log_type"`
	ServiceName        string            `mapstructure:"service_name"`
	Port               string            `mapstructure:"port"`
	Version            string            `mapstructure:"version"`
	CorsMaxAgeHours    time.Duration     `mapstructure:"cors_max_age_hours"`
	RobotsUrlPath      string            `mapstructure:"robots_url_path"`
	MaxBodySize        int64             `mapstructure:"max_body_size"`
	CacheSettings      *CacheConfig      `mapstructure:"cache"`
	DbSettings         *DatabaseConfig   `mapstructure:"database"`
	HttpClientSettings *HttpClientConfig `mapstructure:"http_client"`
}

type CacheConfig struct {
	Servers         string        `mapstructure:"servers"`
	TtlForRobotsTxt time.Duration `mapstructure:"ttl_for_robots_txt"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            string        `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
}

type HttpClientConfig struct {
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
}

func MustLoad() *Config {
	viper.AddConfigPath(path.Join("."))
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		slog.Error("can't initialize config file.", slog.String("err", err.Error()))
		os.Exit(1)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		slog.Error("error unmarshalling viper config.", slog.String("err", err.Error()))
		os.Exit(1)
	}

	return &cfg
}
