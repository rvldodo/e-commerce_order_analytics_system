package postgresql

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"e-commerce_order_analytics_system/internal/repository/queries"
	"e-commerce_order_analytics_system/pkg/errors"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

const analyticsQueryTimeout = 2 * time.Minute

func (pg *pgStruct) selectAll(
	ctx context.Context,
	name string,
	dest any,
	query string,
	args ...any,
) (err error) {
	start := time.Now()
	defer func() { logQuery(ctx, name, start, rowCount(dest), err) }()

	dbCtx, cancel := context.WithTimeout(ctx, analyticsQueryTimeout)
	defer cancel()

	if err := pg.db.SelectContext(dbCtx, dest, query, args...); err != nil {
		return errors.Wrap(errors.FailToSelect, err)
	}
	return nil
}

func (pg *pgStruct) GetCustomerCohorts(
	ctx context.Context,
	year int,
) ([]entity.CustomerCohortEntity, error) {
	res := []entity.CustomerCohortEntity{}
	err := pg.selectAll(ctx, "CustomerCohortAnalysisQuery", &res, queries.CustomerCohortAnalysisQuery, year)
	return res, err
}

func (pg *pgStruct) GetProductPerformance(
	ctx context.Context,
) ([]entity.ProductPerformanceEntity, error) {
	res := []entity.ProductPerformanceEntity{}
	err := pg.selectAll(ctx, "ProductPerformanceQuery", &res, queries.ProductPerformanceQuery)
	return res, err
}

func (pg *pgStruct) GetCustomerRFM(ctx context.Context) ([]entity.CustomerRFMEntity, error) {
	res := []entity.CustomerRFMEntity{}
	err := pg.selectAll(ctx, "CustomerRFMSegmentationQuery", &res, queries.CustomerRFMSegmentationQuery)
	return res, err
}

func (pg *pgStruct) GetSalesTrend(
	ctx context.Context,
	days int,
) ([]entity.SalesTrendEntity, error) {
	res := []entity.SalesTrendEntity{}
	err := pg.selectAll(ctx, "SalesTrendAnalysisQuery", &res, queries.SalesTrendAnalysisQuery, days)
	return res, err
}

func (pg *pgStruct) GetInventoryTurnover(
	ctx context.Context,
	days int,
) ([]entity.InventoryTurnoverEntity, error) {
	res := []entity.InventoryTurnoverEntity{}
	err := pg.selectAll(ctx, "InventoryTurnoverQuery", &res, queries.InventoryTurnoverQuery, days)

	var pqErr *pq.Error
	if stderrors.As(err, &pqErr) && pqErr.Code == "42703" {
		return nil, errors.Wrap(errors.FailToSelect, fmt.Errorf(
			"inventory report needs products.stock_quantity, which the current schema does not have; "+
				"add it with a migration first (%s)",
			pqErr.Message,
		))
	}
	return res, err
}

func (pg *pgStruct) GetCustomerPurchasePatterns(
	ctx context.Context,
	year int,
) ([]entity.CustomerPurchasePatternEntity, error) {
	res := []entity.CustomerPurchasePatternEntity{}
	err := pg.selectAll(ctx, "CustomerPurchasePatternQuery", &res, queries.CustomerPurchasePatternQuery, year)
	return res, err
}
