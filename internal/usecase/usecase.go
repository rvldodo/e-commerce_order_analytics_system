package usecase

import (
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/internal/usecase/report"
)

type UsecaseStruct struct {
	Report report.ReportInterface
}

func New(repo *repository.RepoStruct) *UsecaseStruct {
	return &UsecaseStruct{
		Report: report.New(repo),
	}
}
