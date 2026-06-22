package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3Config holds S3 connection configuration
type S3Config struct {
	BucketName string `json:"bucket_name"`
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"`
	Endpoint   string `json:"endpoint"`
	Region     string `json:"region"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
}

type SIDStoreConfig struct {
	BaseURL        string `json:"base_url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// Config holds the entire configuration structure
type Config struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	S3              S3Config       `json:"s3"`
	Database        DatabaseConfig `json:"database"`
	UIDDatabase     DatabaseConfig `json:"uid_database"`
	Opt360Database  DatabaseConfig `json:"opt360_database"`
	SIDStore        SIDStoreConfig `json:"sid_store"`
}

var (
	config     *Config
	configOnce sync.Once
	configErr  error
)

// Load reads and validates the configuration from the given file path.
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file '%s': %w", path, err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	// Basic validation
	if cfg.S3.AccessKey == "" || cfg.S3.SecretKey == "" || cfg.S3.Endpoint == "" {
		return nil, fmt.Errorf("s3 'access_key', 'secret_key', and 'endpoint' must be set in config file")
	}
	if cfg.S3.BucketName == "" {
		return nil, fmt.Errorf("s3 'bucket_name' must be set in config file")
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080 
	}
	if cfg.SIDStore.BaseURL == "" {
		cfg.SIDStore.BaseURL = "http://localhost:9001"
	}
	if cfg.SIDStore.TimeoutSeconds <= 0 {
		cfg.SIDStore.TimeoutSeconds = 15
	}
	
	// Database validation
	if cfg.Database.User == "" || cfg.Database.Password == "" || cfg.Database.Host == "" {
		return nil, fmt.Errorf("database 'user', 'password', and 'host' must be set in config file")
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 3306 // Default MySQL port
	}

	// UID Database validation
	if cfg.UIDDatabase.User == "" || cfg.UIDDatabase.Password == "" || cfg.UIDDatabase.Host == "" {
		return nil, fmt.Errorf("uid_database 'user', 'password', and 'host' must be set in config file")
	}
	if cfg.UIDDatabase.Port == 0 {
		cfg.UIDDatabase.Port = 3306 // Default MySQL port
	}

	// Opt360 Database validation
	if cfg.Opt360Database.User == "" || cfg.Opt360Database.Password == "" || cfg.Opt360Database.Host == "" {
		return nil, fmt.Errorf("opt360_database 'user', 'password', and 'host' must be set in config file")
	}
	if cfg.Opt360Database.Port == 0 {
		cfg.Opt360Database.Port = 3306 // Default MySQL port
	}

	return &cfg, nil
}

// LoadConfig loads configuration from config.json file
func LoadConfig() (*Config, error) {
	configOnce.Do(func() {
		config, configErr = Load("config.json")
	})

	return config, configErr
}

// GetDefaultS3Config returns the S3 configuration from config.json
func GetDefaultS3Config() S3Config {
	cfg, err := LoadConfig()
    if err != nil {
		fmt.Println("Error loading config:", err)
		return S3Config{}
	}

	return cfg.S3
}

// NewS3Client creates a new S3 client with the given configuration
func NewS3Client(cfg S3Config) (*s3.S3, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(cfg.Region),
		Endpoint:         aws.String(cfg.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
	})
	if err != nil {
		return nil, err
	}

	return s3.New(sess), nil
}


