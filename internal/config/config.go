package config

import (
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres               PostgresConfig   `yaml:"postgres"`
	Kafka                  KafkaConfig      `yaml:"kafka"`
	Env                    string           `yaml:"env" env:"ENV" env-default:"local"`
	MasterSecret           string           `yaml:"master_secret"`
	Redis                  RedisConfig      `yaml:"redis"`
	HTTPServer             HTTPServerConfig `yaml:"http_server"`
	Caching                CachingConfig    `yaml:"caching"`
	GRPC                   GRPCConfig       `yaml:"grpc"`
	TokenTTL               time.Duration    `yaml:"token_ttl" env-default:"1h"`
	RefreshTokenTTL        time.Duration    `yaml:"refresh_token_ttl" env-default:"720h"`
	ReissueRefreshTokenTTL time.Duration    `yaml:"reissue_refresh_token_ttl" env-default:"24h"`
	Hasher                 HasherConfig     `yaml:"hasher"`
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
	WorkerLimit uint64 `yaml:"worker_limit" env-default:"16"`
}

type PostgresConfig struct {
	Direct  PostgresNodeConfig `yaml:"direct" env-prefix:"DB_DIRECT_"`
	Master  PostgresNodeConfig `yaml:"master" env-prefix:"DB_MASTER_"`
	Replica PostgresNodeConfig `yaml:"replicas" env-prefix:"DB_REPLICA_"`
}

type PostgresNodeConfig struct {
	Host     string `yaml:"host" env:"HOST" env-required:"true"`
	User     string `yaml:"user" env:"USER" env-required:"true"`
	Password string `yaml:"password" env:"PASSWORD" env-required:"true"`
	Database string `yaml:"database" env:"NAME" env-required:"true"`
	SSLMode  string `yaml:"ssl_mode" env-default:"disable"`
	Port     int    `yaml:"port" env:"PORT" env-required:"true"`
}

type RedisConfig struct {
	Addresses        []string      `yaml:"addresses"`
	OperationTimeout time.Duration `yaml:"operation_timeout" env-default:"10ms"`
}

type CachingConfig struct {
	Enabled         bool          `yaml:"enabled" env-default:"false"`
	AppTTL          time.Duration `yaml:"app_ttl" env-default:"1m"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl" env-default:"24h"`
}

type KafkaConfig struct {
	BootstrapServers      string        `yaml:"bootstrap_servers" env:"KAFKA_BOOTSTRAP_SERVERS" env-default:"localhost:9092" env-required:"true"`
	ClientID              string        `yaml:"client_id" env:"KAFKA_CLIENT_ID" env-default:"auth-producer:" env-required:"true"`
	TopicAppKey           string        `yaml:"topic-app-key" env:"KAFKA_TOPIC_APP_KEY" env-default:"auth-app-key-v1" env-required:"true"`
	TopicUserActivity     string        `yaml:"topic-user-activity" env:"KAFKA_TOPIC_USER_ACTIVITY" env-default:"auth-user-activity-v1" env-required:"true"`
	CompressionType       string        `yaml:"compression_type" env-default:"lz4"`
	Acks                  string        `yaml:"acks" env-default:"all"`
	RetryBackoffMs        int           `yaml:"retry_backoff_ms" env-default:"500"`
	Retries               int           `yaml:"retries" env-default:"2000"`
	MessageTimeoutMs      int           `yaml:"message_timeout_ms" env-default:"900000"`
	QueueBufferingMaxMsgs int           `yaml:"queue_buffering_max_msgs" env-default:"100000"`
	ProducerMaxRetries    int           `yaml:"producer_max_retries" env-default:"3"`
	ProducerRetryBackoff  time.Duration `yaml:"producer_retry_backoff" env-default:"10ms"`
	LingerMs              int           `yaml:"linger_ms" env-default:"50"`
	BatchNumMessages      int           `yaml:"batch_num_messages" env-default:"1000"`
	EnableIdempotence     bool          `yaml:"enable_idempotence" env-default:"true"`
	SocketKeepaliveEnable bool          `yaml:"socket_keepalive_enable" env-default:"true"`
}

var (
	cfg  *Config
	once sync.Once
)

func init() {
	// Register the flag in the global set so that 'go test' and other tools
	// using flag.Parse() recognize it and don't fail during initialization.
	if flag.Lookup("config") == nil {
		flag.String("config", "", "path to config file")
	}
}

func MustLoad() *Config {
	once.Do(func() {
		var configPath string

		// Manually look for -config flag to avoid conflicts with other FlagSets
		// used in tools like migrators or test runners.
		for i := 0; i < len(os.Args); i++ {
			arg := os.Args[i]
			if strings.HasPrefix(arg, "-config=") || strings.HasPrefix(arg, "--config=") {
				parts := strings.SplitN(arg, "=", 2)
				configPath = parts[1]
				break
			} else if arg == "-config" || arg == "--config" {
				if i+1 < len(os.Args) {
					configPath = os.Args[i+1]
					break
				}
			}
		}

		if configPath == "" {
			configPath = os.Getenv("CONFIG_PATH")
		}

		if configPath == "" {
			panic("CONFIG_PATH is not set")
		}

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			panic("config file does not exist: " + configPath)
		}

		cfg = &Config{}
		if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
			panic("cannot read config: " + err.Error())
		}
	})

	return cfg
}
