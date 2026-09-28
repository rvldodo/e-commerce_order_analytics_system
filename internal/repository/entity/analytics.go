package entity

import "time"

type CustomerCohortEntity struct {
	CohortMonth           string  `db:"cohort_month"`
	NewCustomers          int64   `db:"new_customers"`
	FirstMonthRevenue     float64 `db:"first_month_revenue"`
	RunningTotalCustomers int64   `db:"running_total_customers"`
	RetainedCustomers     int64   `db:"retained_customers"`
	RetentionRatePct      float64 `db:"retention_rate_pct"`
	NewCustomersChange    *int64  `db:"new_customers_change"`
}

type ProductPerformanceEntity struct {
	ProductID               int64    `db:"product_id"`
	ProductName             string   `db:"product_name"`
	Category                string   `db:"category"`
	ParentCategory          string   `db:"parent_category"`
	TotalRevenue            float64  `db:"total_revenue"`
	UnitsSold               int64    `db:"units_sold"`
	CategoryRevenueRank     int64    `db:"category_revenue_rank"`
	CategoryRevenueSharePct *float64 `db:"category_revenue_share_pct"`
	LastMonth               string   `db:"last_month"`
	LastMonthRevenue        float64  `db:"last_month_revenue"`
	PrevMonthRevenue        float64  `db:"prev_month_revenue"`
	MomRevenueChange        float64  `db:"mom_revenue_change"`
	MomRevenueChangePct     *float64 `db:"mom_revenue_change_pct"`
	IsTop20PctInCategory    bool     `db:"is_top_20_pct_in_category"`
}

type CustomerRFMEntity struct {
	CustomerID     int64   `db:"customer_id"`
	Name           string  `db:"name"`
	Email          string  `db:"email"`
	RecencyDays    int64   `db:"recency_days"`
	FrequencyCount int64   `db:"frequency_count"`
	MonetaryValue  float64 `db:"monetary_value"`
	RScore         int64   `db:"r_score"`
	FScore         int64   `db:"f_score"`
	MScore         int64   `db:"m_score"`
	RFMScore       int64   `db:"rfm_score"`
	Segment        string  `db:"segment"`
}

type SalesTrendEntity struct {
	Day             time.Time `db:"day"`
	DayOfWeek       string    `db:"day_of_week"`
	TotalOrders     int64     `db:"total_orders"`
	Revenue         float64   `db:"revenue"`
	Revenue7dAvg    float64   `db:"revenue_7d_avg"`
	Orders7dAvg     float64   `db:"orders_7d_avg"`
	RevenueVsAvgPct *float64  `db:"revenue_vs_avg_pct"`
	AnomalyFlag     string    `db:"anomaly_flag"`
}

type InventoryTurnoverEntity struct {
	ProductID         int64      `db:"product_id"`
	ProductName       string     `db:"product_name"`
	Category          string     `db:"category"`
	StockQuantity     int64      `db:"stock_quantity"`
	UnitsSold90d      int64      `db:"units_sold_90d"`
	DailySalesRate    float64    `db:"daily_sales_rate"`
	DaysUntilStockout *float64   `db:"days_until_stockout"`
	LastOrderDate     *time.Time `db:"last_order_date"`
	StockStatus       string     `db:"stock_status"`
	ReorderQuantity   int64      `db:"reorder_quantity"`
}

type CustomerPurchasePatternEntity struct {
	CustomerID              int64    `db:"customer_id"`
	Name                    string   `db:"name"`
	Email                   string   `db:"email"`
	TotalOrders             int64    `db:"total_orders"`
	AvgDaysBetweenOrders    *float64 `db:"avg_days_between_orders"`
	StddevDaysBetweenOrders *float64 `db:"stddev_days_between_orders"`
	MostPurchasedCategory   *string  `db:"most_purchased_category"`
	AvgOrderValue           float64  `db:"avg_order_value"`
	SpendingTrend           string   `db:"spending_trend"`
	LifetimeDays            int64    `db:"lifetime_days"`
}
