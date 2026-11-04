package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
)

// TestS3 runs against a real S3-compatible bucket (R2, MinIO...) when
// S3_TEST_ENDPOINT, S3_TEST_BUCKET, S3_TEST_ACCESS_KEY_ID and
// S3_TEST_SECRET_ACCESS_KEY are set (S3_TEST_REGION defaults to auto).
func TestS3(t *testing.T) {
	c := S3Config{Endpoint: os.Getenv("S3_TEST_ENDPOINT"), Region: os.Getenv("S3_TEST_REGION"), Bucket: os.Getenv("S3_TEST_BUCKET"),
		AccessKeyID: os.Getenv("S3_TEST_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("S3_TEST_SECRET_ACCESS_KEY")}
	if c.Endpoint == "" {
		t.Skip("S3_TEST_ENDPOINT not set")
	}
	if c.Region == "" {
		c.Region = "auto"
	}
	ctx := context.Background()
	store, err := NewS3(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	key, data := "resume/"+uuid.NewString()+".pdf", []byte("%PDF-1.4 test")
	if err := store.Put(ctx, key, "application/pdf", data); err != nil {
		t.Fatal(err)
	}
	got, contentType, err := store.Get(ctx, key)
	if err != nil || !bytes.Equal(got, data) || contentType != "application/pdf" {
		t.Fatalf("get: %q %q %v", got, contentType, err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete again: %v", err)
	}
	if err := store.Put(ctx, "../etc/passwd", "text/plain", data); err == nil {
		t.Fatal("a bad key was stored")
	}
	if _, err := NewS3(ctx, S3Config{Endpoint: c.Endpoint, Region: c.Region, Bucket: "missing-" + uuid.NewString()[:8],
		AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey}); err == nil {
		t.Fatal("a missing bucket was accepted")
	}
}
