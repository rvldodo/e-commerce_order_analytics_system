package job

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository"
	"sync"
)

type jobStruct struct {
	repo *repository.RepoStruct
	mu   sync.Mutex
}

type JobInterface interface {
	WeeklySalesReport(ctx context.Context) error
	LowStockAlerts(ctx context.Context) error
	CustomerChurnRiskAnalysis(ctx context.Context) error
	MonthlyRevenueBreakdown(ctx context.Context) error
}

func NewJob(repo *repository.RepoStruct) JobInterface {
	return &jobStruct{
		repo: repo,
	}
}
