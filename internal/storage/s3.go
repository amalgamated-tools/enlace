package storage

import (
	"context"
	"errors"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Compile-time check that S3Storage implements Storage interface.
var _ Storage = (*S3Storage)(nil)

// S3Storage implements the Storage interface using S3 or S3-compatible services.
// It supports custom endpoints for services like MinIO, Backblaze B2, and Cloudflare R2.
type S3Storage struct {
	client     *s3.Client
	bucket     string
	pathPrefix string
	region     string
}

// S3Config holds the configuration for connecting to an S3 or S3-compatible service.
type S3Config struct {
	Endpoint   string // Custom endpoint URL (for MinIO, Backblaze, R2, etc.)
	Bucket     string // Bucket name
	AccessKey  string // Access key ID
	SecretKey  string // Secret access key
	Region     string // AWS region
	PathPrefix string // Optional prefix for all keys (e.g., "uploads/")
}

// NewS3Storage creates a new S3Storage instance with the given configuration.
// It validates the configuration and establishes a connection to the S3 service.
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("bucket name is required")
	}
	if cfg.AccessKey == "" {
		return nil, errors.New("access key is required")
	}
	if cfg.SecretKey == "" {
		return nil, errors.New("secret key is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1" // Default region
	}

	// Create static credentials provider
	credProvider := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	// Build AWS config options
	configOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credProvider),
	}

	// Load the AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, err
	}

	// Build S3 client options
	s3Opts := []func(*s3.Options){
		func(o *s3.Options) {
			// Use path-style URLs (required for S3-compatible services)
			o.UsePathStyle = true
		},
	}

	// Add custom endpoint if specified
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	// Create the S3 client
	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Storage{
		client:     client,
		bucket:     cfg.Bucket,
		pathPrefix: cfg.PathPrefix,
		region:     cfg.Region,
	}, nil
}

// fullKey returns the full S3 object key with the path prefix applied.
func (s *S3Storage) fullKey(key string) string {
	if s.pathPrefix == "" {
		return key
	}
	return path.Join(s.pathPrefix, key)
}

// Put stores data from reader at the given key in S3.
// It creates the object with the specified content type.
func (s *S3Storage) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.fullKey(key)),
		Body:          reader,
		ContentLength: aws.Int64(size),
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := s.client.PutObject(ctx, input)
	return err
}

// Get retrieves the object at the given key from S3.
// Returns ErrNotFound if the object does not exist.
// The caller is responsible for closing the returned ReadCloser.
func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	}

	output, err := s.client.GetObject(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return output.Body, nil
}

// Delete removes the object at the given key from S3.
// Returns ErrNotFound if the object does not exist.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	// First check if the object exists, as S3 DeleteObject doesn't error on missing objects
	exists, err := s.Exists(ctx, key)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	}

	_, err = s.client.DeleteObject(ctx, input)
	return err
}

// Exists checks if the object at the given key exists in S3.
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	}

	_, err := s.client.HeadObject(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// ValidateConnection checks that the S3 credentials and bucket are valid
// by performing a HeadBucket operation.
func (s *S3Storage) ValidateConnection(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}

// isNotFoundError checks if the error indicates a missing object.
func isNotFoundError(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	// AWS SDK v2 may also return a generic error with "NotFound" in the message
	// for HeadObject operations, so we check for that as well
	var apiErr interface {
		ErrorCode() string
	}
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		if code == "NotFound" || code == "NoSuchKey" {
			return true
		}
	}

	return false
}
