package postgresql

import (
	"context"
	"reflect"
	"time"

	"e-commerce_order_analytics_system/pkg/logger"

	"go.uber.org/zap"
)

func rowCount(v any) int {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Slice {
		return rv.Len()
	}
	return 1
}

func logQuery(ctx context.Context, name string, start time.Time, rows int, err error) {
	fields := []zap.Field{
		zap.String("query", name),
		zap.Duration("duration", time.Since(start).Round(time.Millisecond)),
	}
	if err != nil {
		logger.WithContext(ctx).Error("query failed", append(fields, zap.Error(err))...)
		return
	}
	logger.WithContext(ctx).Info("query executed", append(fields, zap.Int("rows", rows))...)
}
