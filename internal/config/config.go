package config

import (
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string           `yaml:"env" env:"ENV" env-default:"local"`
	StoragePath string           `yaml:"storage_path" env:"STORAGE_PATH" env-required:"true"`
	HTTPServer  HTTPServerConfig `yaml:"http_server"`
	GRPC        GRPCConfig       `yaml:"grpc"`
	TokenTTL    time.Duration    `yaml:"token_ttl" env-default:"1h"`
}

type HTTPServerConfig struct {
	Address      string `yaml:"address" env:"HTTP_PORT" env-default:"localhost:8080"`
	Timeout      int    `yaml:"timeout" env-default:"3"`
	ReadTimeout  int    `yaml:"read_timeout" env-default:"5"`
	WriteTimeout int    `yaml:"write_timeout" env-default:"10"`
	IdleTimeout  int    `yaml:"idle_timeout" env-default:"120"`
}

type GRPCConfig struct {
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

func MustLoad() *Config {
	// You can optionally check for a config file path via env or flag
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		panic("CONFIG_PATH is not set")
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config: " + err.Error())
	}

	return &cfg
}
