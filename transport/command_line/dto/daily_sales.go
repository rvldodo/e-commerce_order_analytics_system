package dto

import (
	"encoding/json"
	"time"
)

type DailySalesSummaryResult struct {
	ReportType  string                `json:"report_type"`
	Date        string                `json:"date"`
	Data        DailySalesSummaryData `json:"data"`
	GeneratedAt time.Time             `json:"generated_at"`
}

type DailySalesSummaryData struct {
	TotalRevenue      json.Number `json:"total_revenue"`
	TotalOrders       int64       `json:"total_orders"`
	AverageOrderValue json.Number `json:"average_order_value"`
	TopCategory       *string     `json:"top_category"`
}
