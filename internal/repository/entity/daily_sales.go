package entity

type DailySalesSummaryEntity struct {
	TotalOrders       int64   `db:"total_orders"`
	TotalRevenue      string  `db:"total_revenue"`
	AverageOrderValue string  `db:"average_order_value"`
	TopCategory       *string `db:"top_category"`
}
