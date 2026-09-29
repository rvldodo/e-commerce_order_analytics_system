package report

import (
	"e-commerce_order_analytics_system/internal/repository/queries"
	"fmt"
	"strings"
	"time"
)

type Statement struct {
	Type  Type
	Query string
	SQL   string
	Args  []any
}

func Plan(p Param) (Statement, error) {
	if err := p.Validate(); err != nil {
		return Statement{}, err
	}

	st := Statement{Type: p.Type}
	switch p.Type {
	case CustomerCohort:
		st.Query, st.SQL, st.Args = "CustomerCohortAnalysisQuery", queries.CustomerCohortAnalysisQuery, []any{
			p.Year,
		}
	case ProductPerformance:
		st.Query, st.SQL = "ProductPerformanceQuery", queries.ProductPerformanceQuery
	case RFMSegmentation:
		st.Query, st.SQL = "CustomerRFMSegmentationQuery", queries.CustomerRFMSegmentationQuery
	case SalesTrend:
		st.Query, st.SQL, st.Args = "SalesTrendAnalysisQuery", queries.SalesTrendAnalysisQuery, []any{
			p.Days,
		}
	case InventoryTurnover:
		st.Query, st.SQL, st.Args = "InventoryTurnoverQuery", queries.InventoryTurnoverQuery, []any{
			p.Days,
		}
	case PurchasePatterns:
		st.Query, st.SQL, st.Args = "CustomerPurchasePatternQuery", queries.CustomerPurchasePatternQuery, []any{
			p.Year,
		}
	case DailySalesSummary:
		st.Query, st.SQL, st.Args = "DailySalesSummaryQuery", queries.DailySalesSummaryQuery,
			[]any{p.Date.Format(time.DateOnly)}
	default:
		return Statement{}, fmt.Errorf("unknown report type %q", p.Type)
	}
	st.SQL = dedent(st.SQL)
	return st, nil
}

// String renders the statement as a psql-ready script with the bound
// parameters listed above it.
func (s Statement) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- report: %s (%s)\n", s.Type, s.Query)
	if len(s.Args) == 0 {
		b.WriteString("-- no parameters\n")
	}
	for i, a := range s.Args {
		fmt.Fprintf(&b, "-- $%d = %v\n", i+1, a)
	}
	b.WriteString(s.SQL)
	b.WriteString(";\n")
	return b.String()
}

func dedent(sql string) string {
	lines := strings.Split(strings.Trim(sql, "\n"), "\n")
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, "\t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		if len(l) >= indent && indent > 0 {
			lines[i] = l[indent:]
		}
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
