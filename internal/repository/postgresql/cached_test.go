package postgresql

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"e-commerce_order_analytics_system/pkg/cache"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type countingPG struct {
	PostgresInterface
	calls int
	err   error
}

func (c *countingPG) GetSalesTrend(ctx context.Context, days int) ([]entity.SalesTrendEntity, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	rev := 10.5
	return []entity.SalesTrendEntity{{
		Day: time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC), TotalOrders: int64(days), RevenueVsAvgPct: &rev,
	}}, nil
}

// memStore shares the test clock, so expiry can be driven without sleeping.
type memStore struct {
	now     *time.Time
	entries map[string]memEntry
}

type memEntry struct {
	raw       []byte
	expiresAt time.Time
}

func (m *memStore) Get(key string, dest any) (bool, error) {
	e, ok := m.entries[key]
	if !ok || !m.now.Before(e.expiresAt) {
		return false, nil
	}
	return true, json.Unmarshal(e.raw, dest)
}

func (m *memStore) Set(key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	m.entries[key] = memEntry{raw: raw, expiresAt: m.now.Add(ttl)}
	return err
}

func newTestCache(t *testing.T, inner PostgresInterface, now *time.Time) *cachedPG {
	t.Helper()
	store := &memStore{now: now, entries: map[string]memEntry{}}
	c := NewCached(inner, store, time.Minute, time.UTC).(*cachedPG)
	c.now = func() time.Time { return *now }
	return c
}

func TestCachedWithFileStore(t *testing.T) {
	inner := &countingPG{}
	c := NewCached(inner, cache.NewFileStore(t.TempDir()), time.Minute, time.UTC)
	_, _ = c.GetSalesTrend(context.Background(), 90)
	got, err := c.GetSalesTrend(context.Background(), 90)
	if err != nil || inner.calls != 1 || got[0].TotalOrders != 90 {
		t.Fatalf("calls=%d err=%v rows=%+v", inner.calls, err, got)
	}
}

func TestCachedServesRepeatedQueries(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	inner := &countingPG{}
	c := newTestCache(t, inner, &now)
	ctx := context.Background()

	first, err := c.GetSalesTrend(ctx, 90)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.GetSalesTrend(ctx, 90)
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Fatalf("inner called %d times, want 1", inner.calls)
	}
	if second[0].TotalOrders != 90 || *second[0].RevenueVsAvgPct != 10.5 || !second[0].Day.Equal(first[0].Day) {
		t.Fatalf("cached value differs: %+v", second[0])
	}

	if _, err := c.GetSalesTrend(ctx, 30); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Fatal("different arguments must not share a cache entry")
	}
}

func TestCachedExpiresAndRollsOverDays(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	inner := &countingPG{}
	c := newTestCache(t, inner, &now)
	ctx := context.Background()

	_, _ = c.GetSalesTrend(ctx, 90)
	now = now.Add(2 * time.Minute)
	_, _ = c.GetSalesTrend(ctx, 90)
	if inner.calls != 2 {
		t.Fatalf("expired entry should be reloaded, calls=%d", inner.calls)
	}

	now = time.Date(2026, 9, 29, 23, 59, 50, 0, time.UTC)
	_, _ = c.GetSalesTrend(ctx, 90)
	now = now.Add(20 * time.Second)
	_, _ = c.GetSalesTrend(ctx, 90)
	if inner.calls != 4 {
		t.Fatalf("a new day must not reuse yesterday's result, calls=%d", inner.calls)
	}
}

func TestCachedBypassRefreshes(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	inner := &countingPG{}
	c := newTestCache(t, inner, &now)

	_, _ = c.GetSalesTrend(context.Background(), 90)
	_, _ = c.GetSalesTrend(cache.WithBypass(context.Background()), 90)
	_, _ = c.GetSalesTrend(context.Background(), 90)
	if inner.calls != 2 {
		t.Fatalf("calls=%d, want 2 (bypass reloads, next call hits the refreshed entry)", inner.calls)
	}
}

func TestCachedDoesNotStoreErrors(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	inner := &countingPG{err: errors.New("db down")}
	c := newTestCache(t, inner, &now)

	if _, err := c.GetSalesTrend(context.Background(), 90); err == nil {
		t.Fatal("want error")
	}
	inner.err = nil
	if _, err := c.GetSalesTrend(context.Background(), 90); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Fatal("a failed query must not be cached")
	}
}

func TestNewCachedDisabled(t *testing.T) {
	inner := &countingPG{}
	if NewCached(inner, cache.NewFileStore(t.TempDir()), 0, nil) != PostgresInterface(inner) {
		t.Fatal("ttl 0 should return the inner repository unchanged")
	}
}

func TestRowCount(t *testing.T) {
	rows := []int{1, 2, 3}
	if rowCount(rows) != 3 || rowCount(&rows) != 3 || rowCount(struct{}{}) != 1 {
		t.Fatal("rowCount mismatch")
	}
	var nilPtr *[]int
	if rowCount(nilPtr) != 0 {
		t.Fatal("nil pointer should count as 0")
	}
}
