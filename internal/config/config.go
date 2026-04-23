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
}

type GRPCConfig struct {
	Port    int           `yaml:"port" env-default:"9092"`
	Timeout time.Duration `yaml:"timeout" env-default:"5s"`
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

	return nil
}

func (c Config) GRPCAddr() string {
	return fmt.Sprintf(":%d", c.GRPC.Port)
}

func (c Config) HTTPAddr() string {
	return fmt.Sprintf(":%d", c.HTTP.Port)
}
