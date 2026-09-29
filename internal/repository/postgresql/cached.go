package postgresql

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"e-commerce_order_analytics_system/pkg/cache"
	"e-commerce_order_analytics_system/pkg/logger"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// cacheVersion is part of every key; bump it when an entity's fields change.
const cacheVersion = "v1"

type cachedPG struct {
	inner PostgresInterface
	store cache.Store
	ttl   time.Duration
	loc   *time.Location
	now   func() time.Time
}

// NewCached wraps inner with a result cache. Keys include the current day in
// loc because several queries are relative to now() (last 90 days, last
// completed month), so a result never outlives the day it was computed for.
func NewCached(
	inner PostgresInterface,
	store cache.Store,
	ttl time.Duration,
	loc *time.Location,
) PostgresInterface {
	if ttl <= 0 || store == nil {
		return inner
	}
	if loc == nil {
		loc = time.UTC
	}
	return &cachedPG{inner: inner, store: store, ttl: ttl, loc: loc, now: time.Now}
}

func load[T any](
	c *cachedPG,
	ctx context.Context,
	name string,
	args []any,
	fetch func() (T, error),
) (T, error) {
	key := fmt.Sprintf("%s|%s|%s|%v", cacheVersion, name, c.now().In(c.loc).Format(time.DateOnly), args)

	if !cache.Bypassed(ctx) {
		start := time.Now()
		var cached T
		hit, err := c.store.Get(key, &cached)
		if err != nil {
			logger.WithContext(ctx).Warn("cache read failed", zap.String("query", name), zap.Error(err))
		}
		if hit {
			logger.WithContext(ctx).Info("query served from cache",
				zap.String("query", name),
				zap.Duration("duration", time.Since(start).Round(time.Millisecond)),
				zap.Int("rows", rowCount(cached)),
			)
			return cached, nil
		}
	}

	v, err := fetch()
	if err != nil {
		return v, err
	}
	if err := c.store.Set(key, v, c.ttl); err != nil {
		logger.WithContext(ctx).Warn("cache write failed", zap.String("query", name), zap.Error(err))
	}
	return v, nil
}

func (c *cachedPG) GetCustomerCohorts(
	ctx context.Context,
	year int,
) ([]entity.CustomerCohortEntity, error) {
	return load(c, ctx, "CustomerCohortAnalysisQuery", []any{year},
		func() ([]entity.CustomerCohortEntity, error) { return c.inner.GetCustomerCohorts(ctx, year) })
}

func (c *cachedPG) GetProductPerformance(
	ctx context.Context,
) ([]entity.ProductPerformanceEntity, error) {
	return load(c, ctx, "ProductPerformanceQuery", nil,
		func() ([]entity.ProductPerformanceEntity, error) { return c.inner.GetProductPerformance(ctx) })
}

func (c *cachedPG) GetCustomerRFM(ctx context.Context) ([]entity.CustomerRFMEntity, error) {
	return load(c, ctx, "CustomerRFMSegmentationQuery", nil,
		func() ([]entity.CustomerRFMEntity, error) { return c.inner.GetCustomerRFM(ctx) })
}

func (c *cachedPG) GetSalesTrend(ctx context.Context, days int) ([]entity.SalesTrendEntity, error) {
	return load(c, ctx, "SalesTrendAnalysisQuery", []any{days},
		func() ([]entity.SalesTrendEntity, error) { return c.inner.GetSalesTrend(ctx, days) })
}

func (c *cachedPG) GetInventoryTurnover(
	ctx context.Context,
	days int,
) ([]entity.InventoryTurnoverEntity, error) {
	return load(c, ctx, "InventoryTurnoverQuery", []any{days},
		func() ([]entity.InventoryTurnoverEntity, error) { return c.inner.GetInventoryTurnover(ctx, days) })
}

func (c *cachedPG) GetCustomerPurchasePatterns(
	ctx context.Context,
	year int,
) ([]entity.CustomerPurchasePatternEntity, error) {
	return load(c, ctx, "CustomerPurchasePatternQuery", []any{year},
		func() ([]entity.CustomerPurchasePatternEntity, error) {
			return c.inner.GetCustomerPurchasePatterns(ctx, year)
		})
}

func (c *cachedPG) GetDailySalesSummary(
	ctx context.Context,
	day time.Time,
) (entity.DailySalesSummaryEntity, error) {
	return load(c, ctx, "DailySalesSummaryQuery", []any{day.Format(time.DateOnly)},
		func() (entity.DailySalesSummaryEntity, error) { return c.inner.GetDailySalesSummary(ctx, day) })
}
