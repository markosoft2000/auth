package config

import (
	"flag"
	"os"
	"sync"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env             string           `yaml:"env" env:"ENV" env-default:"local"`
	Postgres        PostgresConfig   `yaml:"postgres"`
	HTTPServer      HTTPServerConfig `yaml:"http_server"`
	GRPC            GRPCConfig       `yaml:"grpc"`
	TokenTTL        time.Duration    `yaml:"token_ttl" env-default:"1h"`
	RefreshTokenTTL time.Duration    `yaml:"refresh_token_ttl" env-default:"30d"`
	Hasher          HasherConfig     `yaml:"hasher"`
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

type HasherConfig struct {
	Memory      uint32 `yaml:"memory" env-default:"65536"` // 64MB
	Iterations  uint32 `yaml:"iterations" env-default:"3"`
	Parallelism uint8  `yaml:"parallelism" env-default:"2"`
	SaltLength  uint32 `yaml:"salt_length" env-default:"16"`
	KeyLength   uint32 `yaml:"key_length" env-default:"32"`
}

type PostgresConfig struct {
	Host     string `yaml:"host" env:"DB_HOST" env-default:"localhost"`
	Port     int    `yaml:"port" env:"DB_PORT" env-default:"5432"`
	User     string `yaml:"user" env:"DB_USER" env-required:"true"`
	Password string `yaml:"password" env:"DB_PASSWORD" env-required:"true"`
	Database string `yaml:"database" env:"DB_NAME" env-required:"true"`
	SSLMode  string `yaml:"ssl_mode" env-default:"disable"`
}

var once sync.Once

func MustLoad() *Config {
	var configPath string
	once.Do(func() {
		flag.StringVar(&configPath, "config", "", "path to config file")
		flag.Parse()
	})

	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
		if configPath == "" {
			panic("CONFIG_PATH is not set")
		}
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
