package worker

import (
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/worker/cronworker"
	"e-commerce_order_analytics_system/transport/worker/job"
	"time"
)

type WorkHandler struct {
	job job.JobInterface
}

func NewWorker(job job.JobInterface) *WorkHandler {
	return &WorkHandler{
		job: job,
	}
}

func (w *WorkHandler) RegisteredWorkers(mgr *cronworker.Manager) *cronworker.Manager {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		logger.Fatalf("load timezone: %v", err)
	}

	must(mgr.Register(cronworker.Job{
		Name:        "Weekly sales report",
		Schedule:    cronworker.WeeklyOn(time.Monday, 7, 0, jakarta),
		Timeout:     30 * time.Minute,
		MaxAttempts: 3,
		Backoff:     30 * time.Second,
		Overlap:     cronworker.Skip,
		Exclusive:   true,
		LockTTL:     2 * time.Hour,
		Handler:     w.job.WeeklySalesReport,
	}))

	must(mgr.Register(cronworker.Job{
		Name:        "Low stock alerts",
		Schedule:    cronworker.DailyAt(7, 0, jakarta),
		Timeout:     30 * time.Minute,
		MaxAttempts: 3,
		Backoff:     30 * time.Second,
		Overlap:     cronworker.Skip,
		Exclusive:   true,
		LockTTL:     2 * time.Hour,
		Handler:     w.job.LowStockAlerts,
	}))

	must(mgr.Register(cronworker.Job{
		Name:        "Customer churn risk analysis",
		Schedule:    cronworker.DailyAt(6, 0, jakarta),
		Timeout:     30 * time.Minute,
		MaxAttempts: 3,
		Backoff:     30 * time.Second,
		Overlap:     cronworker.Skip,
		Exclusive:   true,
		LockTTL:     2 * time.Hour,
		Handler:     w.job.CustomerChurnRiskAnalysis,
	}))

	must(mgr.Register(cronworker.Job{
		Name:        "Monthly revenue breakdown",
		Schedule:    cronworker.MonthlyOn(1, 6, 30, jakarta),
		Timeout:     30 * time.Minute,
		MaxAttempts: 3,
		Backoff:     30 * time.Second,
		Overlap:     cronworker.Skip,
		Exclusive:   true,
		LockTTL:     2 * time.Hour,
		Handler:     w.job.MonthlyRevenueBreakdown,
	}))

	return mgr
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
