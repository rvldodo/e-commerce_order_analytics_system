package postgresql

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"time"

	"database/sql"
	"e-commerce_order_analytics_system/pkg/errors"
	stderrors "errors"
	"github.com/jmoiron/sqlx"
)

type pgStruct struct {
	db *sqlx.DB
}

type PostgresInterface interface {
	GetCustomerCohorts(ctx context.Context, year int) ([]entity.CustomerCohortEntity, error)
	GetProductPerformance(ctx context.Context) ([]entity.ProductPerformanceEntity, error)
	GetCustomerRFM(ctx context.Context) ([]entity.CustomerRFMEntity, error)
	GetSalesTrend(ctx context.Context, days int) ([]entity.SalesTrendEntity, error)
	GetInventoryTurnover(ctx context.Context, days int) ([]entity.InventoryTurnoverEntity, error)
	GetCustomerPurchasePatterns(
		ctx context.Context,
		year int,
	) ([]entity.CustomerPurchasePatternEntity, error)
	GetDailySalesSummary(ctx context.Context, day time.Time) (entity.DailySalesSummaryEntity, error)
}

func NewPG(db *sqlx.DB) PostgresInterface {
	return &pgStruct{
		db: db,
	}
}

func selectError(err error) error {
	if stderrors.Is(err, sql.ErrNoRows) {
		return errors.New(errors.SQLDataIsNotFound)
	}

	return errors.Wrap(errors.FailToSelect, err)
}
