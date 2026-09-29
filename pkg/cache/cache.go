// Package cache stores JSON-encodable values on disk with an expiry time.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Store interface {
	Get(key string, dest any) (bool, error)
	Set(key string, value any, ttl time.Duration) error
}

type FileStore struct {
	dir string
	now func() time.Time
}

type entry struct {
	Key       string          `json:"key"`
	ExpiresAt time.Time       `json:"expires_at"`
	Value     json.RawMessage `json:"value"`
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir, now: time.Now}
}

func (s *FileStore) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func (s *FileStore) Get(key string, dest any) (bool, error) {
	path := s.path(key)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read cache: %w", err)
	}

	var e entry
	if err := json.Unmarshal(raw, &e); err != nil || e.Key != key {
		_ = os.Remove(path)
		return false, nil
	}
	if !s.now().Before(e.ExpiresAt) {
		_ = os.Remove(path)
		return false, nil
	}
	if err := json.Unmarshal(e.Value, dest); err != nil {
		_ = os.Remove(path)
		return false, nil
	}
	return true, nil
}

func (s *FileStore) Set(key string, value any, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache value: %w", err)
	}
	data, err := json.Marshal(entry{Key: key, ExpiresAt: s.now().Add(ttl), Value: raw})
	if err != nil {
		return fmt.Errorf("encode cache entry: %w", err)
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(s.dir, ".entry-*")
	if err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return os.Rename(tmp.Name(), s.path(key))
}

type bypassKey struct{}

func WithBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassKey{}, true)
}

func Bypassed(ctx context.Context) bool {
	v, _ := ctx.Value(bypassKey{}).(bool)
	return v
}
