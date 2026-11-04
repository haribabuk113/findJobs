package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"findJobs/internal/ports"
)

// S3Config is an S3-compatible bucket: Cloudflare R2
// (https://<account id>.r2.cloudflarestorage.com, region "auto"), Internxt,
// MinIO or AWS itself.
type S3Config struct {
	Endpoint, Region, Bucket, AccessKeyID, SecretAccessKey string
}

// S3 stores files in one private bucket. Nothing is public: images are
// served by GET /files/img and résumés by the gated routes, both through
// this app, so the bucket needs no public access or CORS.
type S3 struct {
	client *s3.Client
	bucket string
}

// NewS3 creates the S3-compatible adapter and checks that the bucket is reachable.
func NewS3(ctx context.Context, c S3Config) (ports.ObjectStore, error) {
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(c.Endpoint),
		Region:       c.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretAccessKey, ""),
		// Path-style (endpoint/bucket/key) works on R2, Internxt and MinIO alike.
		UsePathStyle: true,
		// The SDK's default CRC checksums on every request aren't supported by
		// every S3-compatible store; send them only where the API requires them.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.Bucket)}); err != nil {
		return nil, fmt.Errorf("storage bucket %q: %w", c.Bucket, err)
	}
	return &S3{client: client, bucket: c.Bucket}, nil
}

func (s *S3) Put(ctx context.Context, key, contentType string, data []byte) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("invalid storage key %q", key)
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(data),
		ContentType: aws.String(contentType), ContentLength: aws.Int64(int64(len(data))),
	})
	return err
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, string, error) {
	if !keyPattern.MatchString(key) {
		return nil, "", ErrNotFound
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if _, missing := errors.AsType[*types.NoSuchKey](err); missing {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	// The type comes from the key, like the local adapter: never from stored metadata.
	return data, contentTypes[filepath.Ext(key)], err
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if !keyPattern.MatchString(key) {
		return nil
	}
	// S3 deletes are idempotent: a missing key is not an error.
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}
