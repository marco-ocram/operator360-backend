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

// Config holds the entire configuration structure
type Config struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	S3 S3Config `json:"s3"`
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
		cfg.Server.Port = 8080 // Default port
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
		// Fallback to hardcoded values if config file can't be loaded
		return S3Config{
			BucketName: "prd-dsw-bronze-0",
			AccessKey:  "Z3GDXXKL70ZU5LEU60EC",
			SecretKey:  "ZObwRRwsA5SBwNC84vd0Cs7UYswyBkaJpT6nyBqw",
			Endpoint:   "http://10.10.103.14:423",
			Region:     "us-east-1",
		}
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

// NewS3ClientFromConfig creates a new S3 client using Config loaded from Load() function
func NewS3ClientFromConfig(cfg *Config) (*s3.S3, error) {
	return NewS3Client(cfg.S3)
}
