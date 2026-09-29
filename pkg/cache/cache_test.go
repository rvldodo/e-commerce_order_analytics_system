package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*FileStore, *time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	s := NewFileStore(t.TempDir())
	s.now = func() time.Time { return now }
	return s, &now
}

func TestFileStoreRoundTrip(t *testing.T) {
	s, _ := newTestStore(t)
	type row struct {
		Name  string
		Value *float64
		Day   time.Time
	}
	v := 12.5
	in := []row{{"a", &v, time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC)}, {"b", nil, time.Time{}}}

	if err := s.Set("k", in, time.Minute); err != nil {
		t.Fatal(err)
	}
	var out []row
	hit, err := s.Get("k", &out)
	if err != nil || !hit {
		t.Fatalf("hit=%v err=%v, want hit", hit, err)
	}
	if len(out) != 2 || out[0].Name != "a" || *out[0].Value != 12.5 || out[1].Value != nil ||
		!out[0].Day.Equal(in[0].Day) {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestFileStoreMiss(t *testing.T) {
	s, _ := newTestStore(t)
	var out []int
	hit, err := s.Get("absent", &out)
	if err != nil || hit {
		t.Fatalf("hit=%v err=%v, want miss", hit, err)
	}
}

func TestFileStoreExpires(t *testing.T) {
	s, now := newTestStore(t)
	if err := s.Set("k", 1, time.Minute); err != nil {
		t.Fatal(err)
	}

	*now = now.Add(59 * time.Second)
	var out int
	if hit, _ := s.Get("k", &out); !hit {
		t.Fatal("want hit before expiry")
	}

	*now = now.Add(time.Second)
	if hit, _ := s.Get("k", &out); hit {
		t.Fatal("want miss at expiry")
	}
	if _, err := os.Stat(s.path("k")); !os.IsNotExist(err) {
		t.Fatal("expired entry should be removed")
	}
}

func TestFileStoreZeroTTLDoesNotStore(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Set("k", 1, 0); err != nil {
		t.Fatal(err)
	}
	var out int
	if hit, _ := s.Get("k", &out); hit {
		t.Fatal("ttl 0 should disable caching")
	}
}

func TestFileStoreCorruptEntryIsMiss(t *testing.T) {
	s, _ := newTestStore(t)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path("k"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out int
	hit, err := s.Get("k", &out)
	if err != nil || hit {
		t.Fatalf("hit=%v err=%v, want silent miss", hit, err)
	}
}

func TestFileStoreKeysDoNotCollide(t *testing.T) {
	s, _ := newTestStore(t)
	_ = s.Set("a", "first", time.Minute)
	_ = s.Set("b", "second", time.Minute)
	var out string
	if _, _ = s.Get("a", &out); out != "first" {
		t.Fatalf("got %q", out)
	}
	if filepath.Dir(s.path("a")) != s.dir {
		t.Fatal("entries must live in the cache dir")
	}
}

func TestBypass(t *testing.T) {
	if Bypassed(context.Background()) {
		t.Fatal("plain context must not bypass")
	}
	if !Bypassed(WithBypass(context.Background())) {
		t.Fatal("WithBypass must bypass")
	}
}
