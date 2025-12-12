package config

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3Config holds S3 connection configuration
type S3Config struct {
	BucketName string
	AccessKey  string
	SecretKey  string
	Endpoint   string
	Region     string
}

// GetDefaultS3Config returns the default S3 configuration
func GetDefaultS3Config() S3Config {
	return S3Config{
		BucketName: "prd-dsw-bronze-0",
		AccessKey:  "Z3GDXXKL70ZU5LEU60EC",
		SecretKey:  "ZObwRRwsA5SBwNC84vd0Cs7UYswyBkaJpT6nyBqw",
		Endpoint:   "http://10.10.103.14:423",
		Region:     "us-east-1",
	}
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
