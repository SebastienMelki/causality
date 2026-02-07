package warehouse

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// S3Client handles S3/MinIO operations.
type S3Client struct {
	client *s3.Client
	config S3Config
	logger *slog.Logger
}

// NewS3Client creates a new S3 client.
func NewS3Client(ctx context.Context, cfg S3Config, logger *slog.Logger) (*S3Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create AWS config with custom endpoint
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.UsePathStyle
	})

	s3Client := &S3Client{
		client: client,
		config: cfg,
		logger: logger.With("component", "s3-client"),
	}

	logger.Info("S3 client created",
		"endpoint", cfg.Endpoint,
		"bucket", cfg.Bucket,
		"region", cfg.Region,
	)

	return s3Client, nil
}

// RawClient returns the underlying AWS S3 client for use by other modules
// (e.g., compaction) that need direct S3 API access.
func (c *S3Client) RawClient() *s3.Client {
	return c.client
}

// EnsureBucket creates the bucket if it doesn't exist.
func (c *S3Client) EnsureBucket(ctx context.Context) error {
	// Check if bucket exists
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.config.Bucket),
	})
	if err == nil {
		c.logger.Debug("bucket exists", "bucket", c.config.Bucket)
		return nil
	}

	// Create bucket
	c.logger.Info("creating bucket", "bucket", c.config.Bucket)
	_, err = c.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(c.config.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	c.logger.Info("bucket created", "bucket", c.config.Bucket)
	return nil
}

// Upload uploads data to S3.
func (c *S3Client) Upload(ctx context.Context, key string, data []byte) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.config.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/x-parquet"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	c.logger.Debug("uploaded to S3",
		"key", key,
		"size_bytes", len(data),
	)

	return nil
}

// GenerateKey generates an S3 key for the given partition.
// Format: {prefix}/app_id={app}/year={y}/month={m}/day={d}/hour={h}/events_{uuid}.parquet.
func (c *S3Client) GenerateKey(appID string, year, month, day, hour int) string {
	fileUUID := uuid.New().String()
	return fmt.Sprintf(
		"%s/app_id=%s/year=%d/month=%02d/day=%02d/hour=%02d/events_%s.parquet",
		c.config.Prefix,
		appID,
		year,
		month,
		day,
		hour,
		fileUUID,
	)
}

// HealthCheck performs a health check on the S3 connection.
func (c *S3Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.config.Bucket),
	})
	if err != nil {
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	return nil
}

// ListPartitions lists all partitions in the bucket.
func (c *S3Client) ListPartitions(ctx context.Context, appID string) ([]string, error) {
	prefix := fmt.Sprintf("%s/app_id=%s/", c.config.Prefix, appID)

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(c.config.Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})

	var partitions []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list partitions: %w", err)
		}

		for _, cp := range page.CommonPrefixes {
			if cp.Prefix != nil {
				partitions = append(partitions, *cp.Prefix)
			}
		}
	}

	return partitions, nil
}
