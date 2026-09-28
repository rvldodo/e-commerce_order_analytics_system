package auth

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/pkg/jwt"
	"e-commerce_order_analytics_system/transport/http/dto"
)

type authStruct struct {
	repo  *repository.RepoStruct
	token *jwt.Tokenizer
}

type AuthInterface interface {
	Login(ctx context.Context, req dto.LoginParam) (dto.LoginResult, error)

	VerifyAccessToken(
		ctx context.Context,
		param dto.VerifyAccessTokenParam,
	) (dto.VerifyAccessTokenResult, error)
}

func New(repo *repository.RepoStruct, token *jwt.Tokenizer) AuthInterface {
	return &authStruct{
		repo:  repo,
		token: token,
	}
}
