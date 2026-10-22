// Package storage holds ObjectStore adapters. Local disk serves development
// (and a single server); the S3-compatible adapter for Internxt is added once
// credentials exist to test it against.
package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"findJobs/internal/ports"
)

// keyPattern is the only key shape stored or served: generated ids, never
// user-supplied names (system requirements §18), so no path tricks are possible.
var keyPattern = regexp.MustCompile(`^img/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)

var contentTypes = map[string]string{".jpg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}

// ErrNotFound is returned for unknown or malformed keys.
var ErrNotFound = errors.New("file not found")

// Local stores files under a root directory.
type Local struct {
	root string
}

// NewLocal creates the local-disk adapter, creating root if needed.
func NewLocal(root string) (ports.ObjectStore, error) {
	if err := os.MkdirAll(filepath.Join(root, "img"), 0o750); err != nil {
		return nil, fmt.Errorf("storage dir: %w", err)
	}
	return &Local{root: root}, nil
}

func (l *Local) Put(_ context.Context, key, _ string, data []byte) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("invalid storage key %q", key)
	}
	// Write to a temp file and rename, so readers never see half a file.
	path := filepath.Join(l.root, key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (l *Local) Get(_ context.Context, key string) ([]byte, string, error) {
	if !keyPattern.MatchString(key) {
		return nil, "", ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(l.root, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	return data, contentTypes[filepath.Ext(key)], err
}

func (l *Local) Delete(_ context.Context, key string) error {
	if !keyPattern.MatchString(key) {
		return nil
	}
	err := os.Remove(filepath.Join(l.root, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
