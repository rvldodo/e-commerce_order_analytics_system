package auth

import (
	"context"
	"e-commerce_order_analytics_system/transport/http/dto"
	"errors"

	apperror "e-commerce_order_analytics_system/pkg/errors"
)

func (au *authStruct) Login(ctx context.Context, req dto.LoginParam) (dto.LoginResult, error) {
	res := dto.LoginResult{}

	customer, err := au.repo.Postgres.CheckEmail(ctx, req.Email)
	if err != nil {
		var ae *apperror.AppError
		if errors.As(err, &ae) && ae.Code == apperror.SQLDataIsNotFound {
			return res, apperror.New(apperror.UnregisteredEmail)
		}
		return res, err
	}

	tokenDetail, err := au.token.GenerateTokenDetail(customer.ID)
	if err != nil {
		return res, apperror.Wrap(apperror.Undefined, err)
	}

	res.Customer = dto.CustomerResult{
		ID:      customer.ID,
		Email:   customer.Email,
		Name:    customer.Name,
		Country: customer.Country,
	}
	res.AuthenticationTokens = dto.AuthenticationTokens{
		AccessToken:            tokenDetail.AccessToken,
		RefreshToken:           tokenDetail.RefreshToken,
		AccessTokenExpiryTime:  tokenDetail.AccessTokenExpiredAtTime,
		RefreshTokenExpiryTime: tokenDetail.RefreshTokenExpiredAtTime,
	}

	return res, nil
}
