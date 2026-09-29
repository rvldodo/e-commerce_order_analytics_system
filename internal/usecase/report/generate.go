package report

import (
	"context"
	"e-commerce_order_analytics_system/pkg/export"
	"fmt"
)

func (rs *reportStruct) Generate(ctx context.Context, param Param) (export.Sheet, error) {
	if err := param.Validate(); err != nil {
		return export.Sheet{}, err
	}

	var (
		sheet export.Sheet
		err   error
	)
	switch param.Type {
	case CustomerCohort:
		sheet, err = rs.customerCohort(ctx, param.Year)
	case ProductPerformance:
		sheet, err = rs.productPerformance(ctx)
	case RFMSegmentation:
		sheet, err = rs.rfmSegmentation(ctx)
	case SalesTrend:
		sheet, err = rs.salesTrend(ctx, param.Days)
	case InventoryTurnover:
		sheet, err = rs.inventoryTurnover(ctx, param.Days)
	case PurchasePatterns:
		sheet, err = rs.purchasePatterns(ctx, param.Year)
	case DailySalesSummary:
		sheet, err = rs.dailySalesSummarySheet(ctx, param.Date)
	default:
		err = fmt.Errorf("unknown report type %q", param.Type)
	}
	if err != nil {
		return export.Sheet{}, err
	}

	// Queries are already sorted by what matters most, so the limit keeps the
	// head of the report.
	if param.Limit > 0 && len(sheet.Rows) > param.Limit {
		sheet.Rows = sheet.Rows[:param.Limit]
	}
	return sheet, nil
}

func (rs *reportStruct) customerCohort(ctx context.Context, year int) (export.Sheet, error) {
	rows, err := rs.repo.Postgres.GetCustomerCohorts(ctx, year)
	if err != nil {
		return export.Sheet{}, err
	}

	sheet := export.Sheet{
		Name: "Customer Cohorts",
		Columns: []string{
			"Cohort Month", "New Customers", "First Month Revenue", "Running Total Customers",
			"Retained Next Month", "Retention Rate %", "New Customers Change",
		},
	}
	for _, r := range rows {
		sheet.Rows = append(sheet.Rows, []any{
			r.CohortMonth, r.NewCustomers, r.FirstMonthRevenue, r.RunningTotalCustomers,
			r.RetainedCustomers, r.RetentionRatePct, r.NewCustomersChange,
		})
	}
	return sheet, nil
}

func (rs *reportStruct) productPerformance(ctx context.Context) (export.Sheet, error) {
	rows, err := rs.repo.Postgres.GetProductPerformance(ctx)
	if err != nil {
		return export.Sheet{}, err
	}

	sheet := export.Sheet{
		Name: "Product Performance",
		Columns: []string{
			"Product ID", "Product", "Category", "Parent Category", "Total Revenue", "Units Sold",
			"Category Rank", "Category Share %", "Last Month", "Last Month Revenue",
			"Previous Month Revenue", "MoM Change", "MoM Change %", "Top 20% in Category",
		},
	}
	for _, r := range rows {
		sheet.Rows = append(sheet.Rows, []any{
			r.ProductID,
			r.ProductName,
			r.Category,
			r.ParentCategory,
			r.TotalRevenue,
			r.UnitsSold,
			r.CategoryRevenueRank,
			r.CategoryRevenueSharePct,
			r.LastMonth,
			r.LastMonthRevenue,
			r.PrevMonthRevenue,
			r.MomRevenueChange,
			r.MomRevenueChangePct,
			yesNo(r.IsTop20PctInCategory),
		})
	}
	return sheet, nil
}

func (rs *reportStruct) rfmSegmentation(ctx context.Context) (export.Sheet, error) {
	rows, err := rs.repo.Postgres.GetCustomerRFM(ctx)
	if err != nil {
		return export.Sheet{}, err
	}

	sheet := export.Sheet{
		Name: "RFM Segmentation",
		Columns: []string{
			"Customer ID", "Name", "Email", "Recency Days", "Frequency", "Monetary",
			"R", "F", "M", "RFM Score", "Segment",
		},
	}
	for _, r := range rows {
		sheet.Rows = append(sheet.Rows, []any{
			r.CustomerID, r.Name, r.Email, r.RecencyDays, r.FrequencyCount, r.MonetaryValue,
			r.RScore, r.FScore, r.MScore, r.RFMScore, r.Segment,
		})
	}
	return sheet, nil
}

func (rs *reportStruct) salesTrend(ctx context.Context, days int) (export.Sheet, error) {
	rows, err := rs.repo.Postgres.GetSalesTrend(ctx, days)
	if err != nil {
		return export.Sheet{}, err
	}

	sheet := export.Sheet{
		Name: "Sales Trend",
		Columns: []string{
			"Day", "Day of Week", "Orders", "Revenue", "Revenue 7d Avg", "Orders 7d Avg",
			"Revenue vs Avg %", "Anomaly",
		},
	}
	for _, r := range rows {
		sheet.Rows = append(sheet.Rows, []any{
			r.Day, r.DayOfWeek, r.TotalOrders, r.Revenue, r.Revenue7dAvg, r.Orders7dAvg,
			r.RevenueVsAvgPct, r.AnomalyFlag,
		})
	}
	return sheet, nil
}

func (rs *reportStruct) inventoryTurnover(ctx context.Context, days int) (export.Sheet, error) {
	rows, err := rs.repo.Postgres.GetInventoryTurnover(ctx, days)
	if err != nil {
		return export.Sheet{}, err
	}

	sheet := export.Sheet{
		Name: "Inventory Turnover",
		Columns: []string{
			"Product ID", "Product", "Category", "Stock", fmt.Sprintf("Units Sold %dd", days),
			"Daily Sales Rate", "Days Until Stock-out", "Last Order Date", "Stock Status",
			"Reorder Quantity",
		},
	}
	for _, r := range rows {
		var lastOrder any
		if r.LastOrderDate != nil {
			lastOrder = *r.LastOrderDate
		}
		sheet.Rows = append(sheet.Rows, []any{
			r.ProductID, r.ProductName, r.Category, r.StockQuantity, r.UnitsSold90d,
			r.DailySalesRate, r.DaysUntilStockout, lastOrder, r.StockStatus,
			r.ReorderQuantity,
		})
	}
	return sheet, nil
}

func (rs *reportStruct) purchasePatterns(ctx context.Context, year int) (export.Sheet, error) {
	rows, err := rs.repo.Postgres.GetCustomerPurchasePatterns(ctx, year)
	if err != nil {
		return export.Sheet{}, err
	}

	sheet := export.Sheet{
		Name: "Purchase Patterns",
		Columns: []string{
			"Customer ID", "Name", "Email", "Orders", "Avg Days Between Orders",
			"Std Dev Days Between Orders", "Most Purchased Category", "Avg Order Value",
			"Spending Trend", "Lifetime Days",
		},
	}
	for _, r := range rows {
		sheet.Rows = append(sheet.Rows, []any{
			r.CustomerID, r.Name, r.Email, r.TotalOrders, r.AvgDaysBetweenOrders,
			r.StddevDaysBetweenOrders, r.MostPurchasedCategory, r.AvgOrderValue,
			r.SpendingTrend, r.LifetimeDays,
		})
	}
	return sheet, nil
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
