package report

import (
	"context"
	"e-commerce_order_analytics_system/transport/http/dto"
	"time"
)

func (rs *reportStruct) ReportDailySales(ctx context.Context) (dto.ReportResult, error) {
	var res dto.ReportResult

	now := time.Now()
	date := now.Format("2006-01-02") // YYYY-MM-DD

	res.ReportType = "daily_sales"
	res.Date = date
	res.GeneratedAt = now

	return res, nil
}
