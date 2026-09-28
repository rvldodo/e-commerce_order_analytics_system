package usecase

import (
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/internal/usecase/auth"
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/jwt"
)

type UsecaseStruct struct {
	Auth   auth.AuthInterface
	Report report.ReportInterface
}

func New(repo *repository.RepoStruct, tokenizer *jwt.Tokenizer) *UsecaseStruct {
	return &UsecaseStruct{
		Auth:   auth.New(repo, tokenizer),
		Report: report.New(repo),
	}
}
