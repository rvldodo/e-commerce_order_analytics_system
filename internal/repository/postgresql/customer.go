package postgresql

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"e-commerce_order_analytics_system/internal/repository/queries"
	"time"
)

func (pg *pgStruct) GetCustomerByID(
	ctx context.Context,
	customerID int64,
) (entity.CustomerEntity, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	var res entity.CustomerEntity
	err := pg.db.GetContext(dbCtx, &res, queries.GetCustomerByIDQuery, customerID)
	if err != nil {
		return res, selectError(err)
	}

	return res, nil
}

func (pg *pgStruct) CheckEmail(ctx context.Context, email string) (entity.CustomerEntity, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	var res entity.CustomerEntity
	err := pg.db.GetContext(dbCtx, &res, queries.GetCustomerByEmailQuery, email)
	if err != nil {
		return res, selectError(err)
	}

	return res, nil
}
