package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	ServiceName     string        `yaml:"service_name" env-default:"cart-service"`
	Env             string        `yaml:"env" env-default:"local"`
	Version         string        `yaml:"version" env-default:"dev"`
	LogLevel        string        `yaml:"log_level" env-default:"info"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env-default:"10s"`
	GRPC            GRPCConfig    `yaml:"grpc"`
	HTTP            HTTPConfig    `yaml:"http"`
	OTLP            OTLPConfig    `yaml:"otlp"`
	MongoDB         MongoDBConfig `yaml:"mongodb"`
	TLS             TLSConfig     `yaml:"tls"`
	Redis           RedisConfig   `yaml:"redis"`
	Lock            LockConfig    `yaml:"lock"`
}

type LockConfig struct {
	TTL           time.Duration `yaml:"ttl" env-default:"30s"`
	RetryInterval time.Duration `yaml:"retry_interval" env-default:"100ms"`
	MaxRetries    int           `yaml:"max_retries" env-default:"50"`
}

type GRPCConfig struct {
	Port    int           `yaml:"port" env-default:"9092"`
	Timeout time.Duration `yaml:"timeout" env-default:"5s"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled" env-default:"false"`
	CertFile string `yaml:"cert_file" env-default:""`
	KeyFile  string `yaml:"key_file" env-default:""`
	CAFile   string `yaml:"ca_file" env-default:""`
}

// HTTPConfig holds configuration for the HTTP server.
type HTTPConfig struct {
	Port    int           `yaml:"port" env-default:"8084"`
	Timeout time.Duration `yaml:"timeout" env-default:"5s"`
}

type OTLPConfig struct {
	Endpoint string `yaml:"endpoint" env:"OTLP_ENDPOINT" env-default:"jaeger:4317"`
}

type MongoDBConfig struct {
	URI      string `yaml:"uri" env:"MONGODB_URI" env-default:"mongodb://localhost:27017"`
	DBName   string `yaml:"db_name" env:"MONGODB_DB_NAME" env-default:"shop_cart"`
	Username string `yaml:"username" env:"MONGODB_USERNAME" env-default:""`
	Password string `yaml:"password" env:"MONGODB_PASSWORD" env-default:""`
}

type RedisConfig struct {
	Host     string        `yaml:"host" env:"REDIS_HOST" env-default:"localhost"`
	Port     int           `yaml:"port" env:"REDIS_PORT" env-default:"6379"`
	Password string        `yaml:"password" env:"REDIS_PASSWORD" env-default:""`
	DB       int           `yaml:"db" env:"REDIS_DB" env-default:"0"`
	TTL      time.Duration `yaml:"ttl" env:"REDIS_TTL" env-default:"5m"`
}

// MustLoad is a bootstrap helper for main().
// It reads config path only from CONFIG_PATH env.
func MustLoad() *Config {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		panic("CONFIG_PATH is empty")
	}

	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}

	return cfg
}

// Load reads config from YAML and validates the result.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", path)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// MustLoadByPath keeps bootstrap code short in main().
func MustLoadByPath(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}

	return cfg
}

func (c *Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service_name is required")
	}
	if c.Env == "" {
		return fmt.Errorf("env is required")
	}
	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be in range 1..65535")
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port is required")
	}
	if c.GRPC.Timeout <= 0 {
		return fmt.Errorf("grpc.timeout must be > 0")
	}
	if c.HTTP.Timeout <= 0 {
		return fmt.Errorf("http.timeout must be > 0")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown_timeout must be > 0")
	}
	if c.OTLP.Endpoint == "" {
		return fmt.Errorf("otlp.endpoint is required")
	}
	if c.MongoDB.DBName == "" {
		return fmt.Errorf("mongodb.db_name is required")
	}
	if c.MongoDB.URI == "" {
		return fmt.Errorf("mongodb.uri is required")
	}

	if c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			return fmt.Errorf("tls.cert_file is required when tls.enabled=true")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("tls.key_file is required when tls.enabled=true")
		}
		if c.TLS.CAFile == "" {
			return fmt.Errorf("tls.ca_file is required when tls.enabled=true")
		}
	}

	return nil
}

func (c Config) GRPCAddr() string {
	return fmt.Sprintf(":%d", c.GRPC.Port)
}

func (c Config) HTTPAddr() string {
	return fmt.Sprintf(":%d", c.HTTP.Port)
}
func (c Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}
