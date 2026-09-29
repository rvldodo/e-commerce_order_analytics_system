package report

import (
	"context"
	"e-commerce_order_analytics_system/pkg/export"
	"e-commerce_order_analytics_system/transport/command_line/dto"
	"encoding/json"
	"fmt"
	"time"
)

const DailySalesReportType = "daily_sales"

func Yesterday(now time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(Timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("load timezone %s: %w", Timezone, err)
	}
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day()-1, 0, 0, 0, 0, time.UTC), nil
}

func (rs *reportStruct) DailySalesSummary(
	ctx context.Context,
	day time.Time,
) (dto.DailySalesSummaryResult, error) {
	res := dto.DailySalesSummaryResult{}
	if day.IsZero() {
		return res, fmt.Errorf("date is required")
	}

	row, err := rs.repo.Postgres.GetDailySalesSummary(ctx, day)
	if err != nil {
		return res, err
	}

	res.ReportType = DailySalesReportType
	res.Date = day.Format(time.DateOnly)
	res.Data = dto.DailySalesSummaryData{
		TotalRevenue:      json.Number(row.TotalRevenue),
		TotalOrders:       row.TotalOrders,
		AverageOrderValue: json.Number(row.AverageOrderValue),
		TopCategory:       row.TopCategory,
	}
	res.GeneratedAt = time.Now().UTC().Truncate(time.Second)

	return res, nil
}

func (rs *reportStruct) dailySalesSummarySheet(
	ctx context.Context,
	day time.Time,
) (export.Sheet, error) {
	summary, err := rs.DailySalesSummary(ctx, day)
	if err != nil {
		return export.Sheet{}, err
	}
	return SummarySheet(summary), nil
}

func SummarySheet(summary dto.DailySalesSummaryResult) export.Sheet {
	revenue, _ := summary.Data.TotalRevenue.Float64()
	aov, _ := summary.Data.AverageOrderValue.Float64()
	return export.Sheet{
		Name: "Daily Sales Summary",
		Columns: []string{
			"Date", "Total Revenue", "Total Orders", "Average Order Value", "Top Category",
		},
		Rows: [][]any{{
			summary.Date, revenue, summary.Data.TotalOrders, aov, summary.Data.TopCategory,
		}},
	}
}
