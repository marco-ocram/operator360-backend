// Package s3store wraps the S3 fetch/parse and fetch/put patterns that were
// copy-pasted across handlers: fetch-bytes-then-JSON-unmarshal, fetch-bytes
// -then-parquet-decode, and JSON-marshal-then-put.
package s3store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"opt360-portal-backend/config"
	"opt360-portal-backend/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

const operatorStorePrefix = "opt360Store"

// OperatorFilePath builds the per-regional-office S3 key used for operator
// data files, e.g. "opt360Store/UttarPradesh/operator_high.parquet".
func OperatorFilePath(regionalOffice, filename string) string {
	return operatorStorePrefix + "/" + utils.ToPascalCase(regionalOffice) + "/" + filename
}

// NotFoundError marks a failure to fetch the object itself (missing key,
// unreachable bucket), as opposed to a failure decoding its contents.
// Handlers use IsNotFound to decide between a 404 and a 500 response,
// matching the distinction the original handlers made inline.
type NotFoundError struct{ err error }

func (e *NotFoundError) Error() string { return e.err.Error() }
func (e *NotFoundError) Unwrap() error { return e.err }

// IsNotFound reports whether err originated from a failed object fetch.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}

// FetchBytes downloads the raw object at key.
func FetchBytes(cfg config.S3Config, key string) ([]byte, error) {
	client, err := config.NewS3Client(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating s3 client: %w", err)
	}

	result, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(cfg.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, &NotFoundError{fmt.Errorf("fetching s3://%s/%s: %w", cfg.BucketName, key, err)}
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("reading s3://%s/%s: %w", cfg.BucketName, key, err)
	}
	return data, nil
}

// FetchJSON fetches the object at key and unmarshals it into out.
func FetchJSON(cfg config.S3Config, key string, out interface{}) error {
	data, err := FetchBytes(cfg, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parsing s3://%s/%s as JSON: %w", cfg.BucketName, key, err)
	}
	return nil
}

// PutJSON marshals data as indented JSON and uploads it to key.
func PutJSON(cfg config.S3Config, key string, data interface{}) error {
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling payload for s3://%s/%s: %w", cfg.BucketName, key, err)
	}

	client, err := config.NewS3Client(cfg)
	if err != nil {
		return fmt.Errorf("creating s3 client: %w", err)
	}

	_, err = client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(cfg.BucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("uploading s3://%s/%s: %w", cfg.BucketName, key, err)
	}
	return nil
}

// FetchParquetRows fetches the parquet file at key and decodes every row
// into T. The fetch and decode steps are split out as FetchBytes/
// DecodeParquetRows so callers that need to map fetch failures to 404 and
// decode failures to 500 (as the original handlers did) can call them
// separately instead.
func FetchParquetRows[T any](cfg config.S3Config, key string) ([]T, error) {
	data, err := FetchBytes(cfg, key)
	if err != nil {
		return nil, err
	}
	return DecodeParquetRows[T](data, fmt.Sprintf("s3://%s/%s", cfg.BucketName, key))
}

// DecodeParquetRows decodes parquet-encoded data into a slice of T via a
// JSON marshal/unmarshal round-trip per row (parquet-go's row format isn't
// directly assignable to arbitrary structs). Rows that fail to round-trip
// are skipped, matching prior handler behavior. source is used only for
// error messages.
func DecodeParquetRows[T any](data []byte, source string) ([]T, error) {
	tempFile, err := os.CreateTemp("", "parquet_*.parquet")
	if err != nil {
		return nil, fmt.Errorf("creating temp file for %s: %w", source, err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("writing temp file for %s: %w", source, err)
	}
	tempFile.Close()

	fr, err := local.NewLocalFileReader(tempFile.Name())
	if err != nil {
		return nil, fmt.Errorf("opening parquet file %s: %w", source, err)
	}
	defer fr.Close()

	pr, err := reader.NewParquetReader(fr, nil, 4)
	if err != nil {
		return nil, fmt.Errorf("creating parquet reader for %s: %w", source, err)
	}
	defer pr.ReadStop()

	rawRows, err := pr.ReadByNumber(int(pr.GetNumRows()))
	if err != nil {
		return nil, fmt.Errorf("reading parquet rows from %s: %w", source, err)
	}

	rows := make([]T, 0, len(rawRows))
	for _, raw := range rawRows {
		jsonBytes, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		var row T
		if err := json.Unmarshal(jsonBytes, &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}
