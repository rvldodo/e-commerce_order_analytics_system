package postgresql

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"e-commerce_order_analytics_system/internal/repository/queries"
	"e-commerce_order_analytics_system/pkg/errors"
	"time"
)

func (pg *pgStruct) GetDailySalesSummary(
	ctx context.Context,
	day time.Time,
) (res entity.DailySalesSummaryEntity, err error) {
	start := time.Now()
	defer func() { logQuery(ctx, "DailySalesSummaryQuery", start, 1, err) }()

	dbCtx, cancel := context.WithTimeout(ctx, analyticsQueryTimeout)
	defer cancel()

	err = pg.db.GetContext(dbCtx, &res, queries.DailySalesSummaryQuery, day.Format(time.DateOnly))
	if err != nil {
		return res, errors.Wrap(errors.FailToSelect, err)
	}

	return res, nil
}
