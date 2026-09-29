package report

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/pkg/export"
	"e-commerce_order_analytics_system/transport/command_line/dto"
	"fmt"
	"strings"
	"time"
)

// Types lists every report in the order they are shown to users. Each one
// maps to a query in internal/repository/queries/analytics.go.
var Types = []TypeInfo{
	{
		Type:        CustomerCohort,
		Description: "Monthly cohorts by first order: new customers, first-month revenue, running total, next-month retention",
		UsesYear:    true,
	},
	{
		Type:        ProductPerformance,
		Description: "Product revenue and units (all time), rank and share within category, month-over-month change, top 20% flag",
	},
	{
		Type:        RFMSegmentation,
		Description: "Recency, frequency and monetary scores per customer with Champions / Loyal / At Risk / Lost segments",
	},
	{
		Type:        SalesTrend,
		Description: "Daily orders and revenue with 7-day moving averages and >30% anomaly flags",
		UsesDays:    true,
	},
	{
		Type:        InventoryTurnover,
		Description: "Stock, sales rate, days until stock-out, stock status and reorder quantity (needs products.stock_quantity)",
		UsesDays:    true,
	},
	{
		Type:        PurchasePatterns,
		Description: "Customers with 3+ orders who ordered in the year: order gaps, favourite category, spending trend, lifetime",
		UsesYear:    true,
	},
	{
		Type:        DailySalesSummary,
		Description: "One day's completed revenue, orders, average order value and top category (also: report export → JSON)",
		UsesDate:    true,
	},
}

func Lookup(t Type) (TypeInfo, bool) {
	for _, info := range Types {
		if info.Type == t {
			return info, true
		}
	}
	return TypeInfo{}, false
}

func ParseType(s string) (Type, error) {
	t := Type(strings.ToLower(strings.TrimSpace(s)))
	if _, ok := Lookup(t); ok {
		return t, nil
	}

	names := make([]string, len(Types))
	for i, info := range Types {
		names[i] = string(info.Type)
	}
	return "", fmt.Errorf(
		"unknown report type %q: expected one of %s",
		s,
		strings.Join(names, ", "),
	)
}

// Param describes one report run. Year, Days and Date are only read by the
// report types that use them (see TypeInfo). Date is a calendar day in
// Timezone; its clock part is ignored. Limit 0 returns every row.
type Param struct {
	Type  Type
	Year  int
	Days  int
	Date  time.Time
	Limit int
}

func (p Param) Validate() error {
	info, ok := Lookup(p.Type)
	if !ok {
		return fmt.Errorf("unknown report type %q", p.Type)
	}
	if info.UsesYear && (p.Year < 2000 || p.Year > 2100) {
		return fmt.Errorf("--year must be between 2000 and 2100")
	}
	if info.UsesDays && (p.Days < 1 || p.Days > MaxDays) {
		return fmt.Errorf("--days must be between 1 and %d", MaxDays)
	}
	if info.UsesDate && p.Date.IsZero() {
		return fmt.Errorf("--date is required for %s", p.Type)
	}
	if p.Limit < 0 {
		return fmt.Errorf("--limit must not be negative")
	}
	return nil
}

// Tag identifies the run in file names, e.g. "2024" or "90d".
func (p Param) Tag() string {
	info, _ := Lookup(p.Type)
	switch {
	case info.UsesYear:
		return fmt.Sprint(p.Year)
	case info.UsesDays:
		return fmt.Sprintf("%dd", p.Days)
	case info.UsesDate:
		return p.Date.Format(time.DateOnly)
	}
	return ""
}

type ReportInterface interface {
	Generate(ctx context.Context, param Param) (export.Sheet, error)
	GenerateAll(ctx context.Context, params []Param, concurrency int) []Result
	DailySalesSummary(ctx context.Context, day time.Time) (dto.DailySalesSummaryResult, error)
}

type reportStruct struct {
	repo *repository.RepoStruct
}

func New(repo *repository.RepoStruct) ReportInterface {
	return &reportStruct{
		repo: repo,
	}
}
