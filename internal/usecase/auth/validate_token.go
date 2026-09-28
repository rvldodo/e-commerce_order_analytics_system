package auth

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"e-commerce_order_analytics_system/transport/http/dto"

	apperror "e-commerce_order_analytics_system/pkg/errors"
	"e-commerce_order_analytics_system/pkg/jwt"
)

func (au *authStruct) VerifyAccessToken(
	ctx context.Context,
	param dto.VerifyAccessTokenParam,
) (dto.VerifyAccessTokenResult, error) {
	res := dto.VerifyAccessTokenResult{}

	claimsDetail, err := au.token.VerifyToken(param.AccessToken, jwt.AccessTokenType)
	if err != nil {
		return res, apperror.Wrap(apperror.InvalidJWTTokenAuth, err)
	}
	if !claimsDetail.Valid {
		return res, apperror.New(apperror.InvalidJWTToken)
	}

	var customer entity.CustomerEntity
	customer, err = au.repo.Postgres.GetCustomerByID(ctx, claimsDetail.UserID)
	if err != nil {
		return res, err
	}

	if customer.ID > 0 {
		res.Valid = true
	} else {
		res.Valid = false
	}

	return res, nil
}
