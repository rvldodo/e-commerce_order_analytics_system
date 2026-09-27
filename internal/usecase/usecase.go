package usecase

import (
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/pkg/jwt"
)

type UsecaseStruct struct {
}

func New(repo *repository.RepoStruct, tokenizer *jwt.Tokenizer) *UsecaseStruct {
	return &UsecaseStruct{}
}
