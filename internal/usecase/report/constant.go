package report

// Timezone decides which calendar day an order belongs to. It matches the
// timezone used inside the SQL and by the cron worker.
const Timezone = "Asia/Jakarta"

const (
	DefaultYear = 2024
	DefaultDays = 90
	MaxDays     = 730
)

type Type string

const (
	CustomerCohort     Type = "customer_cohort"
	ProductPerformance Type = "product_performance"
	RFMSegmentation    Type = "rfm_segmentation"
	SalesTrend         Type = "sales_trend"
	InventoryTurnover  Type = "inventory_turnover"
	PurchasePatterns   Type = "purchase_patterns"
	DailySalesSummary  Type = "daily_sales_summary"
)

type TypeInfo struct {
	Type        Type
	Description string
	UsesYear    bool
	UsesDays    bool
	UsesDate    bool
}
