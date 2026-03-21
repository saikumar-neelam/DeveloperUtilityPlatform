package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/devutility/webhookplatform/internal/domain"
)

// Config holds S3 / MinIO connection settings.
type Config struct {
	Bucket    string
	Region    string
	// Endpoint is the custom S3-compatible URL (e.g. "http://localhost:9000" for MinIO).
	// Leave empty to use real AWS S3.
	Endpoint  string
	AccessKey string
	SecretKey string
}

// Storage implements domain.PayloadStorage using S3 or any S3-compatible store.
type Storage struct {
	client *s3.Client
	bucket string
}

func New(ctx context.Context, cfg Config) (*Storage, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// Use static credentials when provided (required for MinIO).
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}

	// Client options: custom endpoint + path-style required for MinIO.
	var clientOpts []func(*s3.Options)
	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // MinIO requires path-style addressing
		})
	}

	return &Storage{
		client: s3.NewFromConfig(awsCfg, clientOpts...),
		bucket: cfg.Bucket,
	}, nil
}

func (s *Storage) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3: upload %q: %w", key, err)
	}
	return nil
}

func (s *Storage) Download(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: download %q: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("s3: read body %q: %w", key, err)
	}
	return data, nil
}

// DeleteByPrefix lists all objects whose key starts with prefix and deletes them in batches.
func (s *Storage) DeleteByPrefix(ctx context.Context, prefix string) error {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("s3: list objects with prefix %q: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    obj.Key,
			}); err != nil {
				return fmt.Errorf("s3: delete object %q: %w", aws.ToString(obj.Key), err)
			}
		}
	}
	return nil
}

// Compile-time interface check.
var _ domain.PayloadStorage = (*Storage)(nil)
