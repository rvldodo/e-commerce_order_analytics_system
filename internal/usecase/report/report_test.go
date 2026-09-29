package report

import (
	"context"
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/internal/repository/entity"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRepo struct {
	delay    time.Duration
	inFlight atomic.Int32
	peak     atomic.Int32
	fail     map[string]error
	summary  entity.DailySalesSummaryEntity
	gotDays  int
	gotDate  time.Time
}

func (f *fakeRepo) enter(name string) error {
	n := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		p := f.peak.Load()
		if n <= p || f.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(f.delay)
	return f.fail[name]
}

func (f *fakeRepo) GetCustomerCohorts(
	ctx context.Context,
	year int,
) ([]entity.CustomerCohortEntity, error) {
	return []entity.CustomerCohortEntity{
			{CohortMonth: "2024-10"},
			{CohortMonth: "2024-11"},
		}, f.enter(
			"cohort",
		)
}

func (f *fakeRepo) GetProductPerformance(
	ctx context.Context,
) ([]entity.ProductPerformanceEntity, error) {
	return []entity.ProductPerformanceEntity{
			{ProductName: "A", IsTop20PctInCategory: true},
		}, f.enter(
			"product",
		)
}

func (f *fakeRepo) GetCustomerRFM(ctx context.Context) ([]entity.CustomerRFMEntity, error) {
	rows := make([]entity.CustomerRFMEntity, 10)
	return rows, f.enter("rfm")
}

func (f *fakeRepo) GetSalesTrend(ctx context.Context, days int) ([]entity.SalesTrendEntity, error) {
	f.gotDays = days
	return []entity.SalesTrendEntity{{DayOfWeek: "Monday"}}, f.enter("trend")
}

func (f *fakeRepo) GetInventoryTurnover(
	ctx context.Context,
	days int,
) ([]entity.InventoryTurnoverEntity, error) {
	return nil, f.enter("inventory")
}

func (f *fakeRepo) GetCustomerPurchasePatterns(
	ctx context.Context,
	year int,
) ([]entity.CustomerPurchasePatternEntity, error) {
	return nil, f.enter("patterns")
}

func (f *fakeRepo) GetDailySalesSummary(
	ctx context.Context,
	day time.Time,
) (entity.DailySalesSummaryEntity, error) {
	f.gotDate = day
	return f.summary, f.enter("summary")
}

func newTestReport(f *fakeRepo) *reportStruct {
	return &reportStruct{repo: &repository.RepoStruct{Postgres: f}}
}

func TestParseType(t *testing.T) {
	if got, err := ParseType(" Sales_Trend "); err != nil || got != SalesTrend {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err := ParseType("nope")
	if err == nil || !strings.Contains(err.Error(), "customer_cohort") {
		t.Fatalf("error should list valid types: %v", err)
	}
}

func TestParamValidate(t *testing.T) {
	day := time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		p    Param
		ok   bool
	}{
		{"cohort ok", Param{Type: CustomerCohort, Year: 2024}, true},
		{"cohort bad year", Param{Type: CustomerCohort, Year: 1999}, false},
		{"trend ok", Param{Type: SalesTrend, Days: 90}, true},
		{"trend zero days", Param{Type: SalesTrend, Days: 0}, false},
		{"trend too many days", Param{Type: SalesTrend, Days: MaxDays + 1}, false},
		{"rfm ignores year", Param{Type: RFMSegmentation, Year: 1}, true},
		{"summary needs date", Param{Type: DailySalesSummary}, false},
		{"summary ok", Param{Type: DailySalesSummary, Date: day}, true},
		{"negative limit", Param{Type: RFMSegmentation, Limit: -1}, false},
		{"unknown type", Param{Type: "x"}, false},
	}
	for _, c := range cases {
		if err := c.p.Validate(); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestParamTag(t *testing.T) {
	day := time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC)
	cases := map[string]Param{
		"2024":       {Type: CustomerCohort, Year: 2024},
		"90d":        {Type: SalesTrend, Days: 90},
		"2024-11-29": {Type: DailySalesSummary, Date: day},
		"":           {Type: RFMSegmentation},
	}
	for want, p := range cases {
		if got := p.Tag(); got != want {
			t.Errorf("%s: Tag() = %q, want %q", p.Type, got, want)
		}
	}
}

func TestYesterdayUsesJakartaDay(t *testing.T) {
	// 17:30 UTC on the 28th is already 00:30 on the 29th in Jakarta.
	got, err := Yesterday(time.Date(2026, 9, 28, 17, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.Format(time.DateOnly) != "2026-09-28" {
		t.Fatalf("got %s, want 2026-09-28", got.Format(time.DateOnly))
	}
}

func TestGenerateMapsRowsAndLimits(t *testing.T) {
	rs := newTestReport(&fakeRepo{})

	sheet, err := rs.Generate(context.Background(), Param{Type: RFMSegmentation, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Rows) != 3 || len(sheet.Rows[0]) != len(sheet.Columns) {
		t.Fatalf("rows=%d cols=%d/%d", len(sheet.Rows), len(sheet.Rows[0]), len(sheet.Columns))
	}

	sheet, err = rs.Generate(context.Background(), Param{Type: ProductPerformance})
	if err != nil {
		t.Fatal(err)
	}
	if last := sheet.Rows[0][len(sheet.Rows[0])-1]; last != "Yes" {
		t.Fatalf("top-20 flag = %v, want Yes", last)
	}
}

func TestGenerateRejectsInvalidParam(t *testing.T) {
	f := &fakeRepo{}
	if _, err := newTestReport(
		f,
	).Generate(context.Background(), Param{Type: SalesTrend}); err == nil {
		t.Fatal("want validation error")
	}
	if f.gotDays != 0 {
		t.Fatal("repository must not be called for an invalid param")
	}
}

func TestGenerateAllKeepsOrderAndIsolatesErrors(t *testing.T) {
	f := &fakeRepo{
		delay: 20 * time.Millisecond,
		fail:  map[string]error{"inventory": errors.New("no stock column")},
	}
	params := []Param{
		{Type: CustomerCohort, Year: 2024},
		{Type: InventoryTurnover, Days: 90},
		{Type: SalesTrend, Days: 30},
		{Type: RFMSegmentation},
		{Type: ProductPerformance},
	}

	results := newTestReport(f).GenerateAll(context.Background(), params, 2)

	if len(results) != len(params) {
		t.Fatalf("got %d results", len(results))
	}
	for i, r := range results {
		if r.Param.Type != params[i].Type {
			t.Fatalf("result %d is %s, want %s", i, r.Param.Type, params[i].Type)
		}
	}
	if results[1].Err == nil || results[0].Err != nil || results[2].Err != nil {
		t.Fatalf(
			"errors not isolated: %v / %v / %v",
			results[0].Err,
			results[1].Err,
			results[2].Err,
		)
	}
	if peak := f.peak.Load(); peak > 2 {
		t.Fatalf("peak concurrency %d exceeds limit 2", peak)
	}
	if peak := f.peak.Load(); peak < 2 {
		t.Fatalf("peak concurrency %d, reports did not run in parallel", peak)
	}
}

func TestGenerateAllRunsFasterThanSequential(t *testing.T) {
	f := &fakeRepo{delay: 50 * time.Millisecond}
	params := []Param{
		{Type: RFMSegmentation},
		{Type: ProductPerformance},
		{Type: CustomerCohort, Year: 2024},
	}

	start := time.Now()
	newTestReport(f).GenerateAll(context.Background(), params, 3)
	if d := time.Since(start); d > 130*time.Millisecond {
		t.Fatalf("took %s, want roughly one delay (50ms)", d)
	}
}

func TestDailySalesSummaryJSON(t *testing.T) {
	top := "Electronics"
	f := &fakeRepo{summary: entity.DailySalesSummaryEntity{
		TotalOrders: 450, TotalRevenue: "125000.50", AverageOrderValue: "277.78", TopCategory: &top,
	}}
	day := time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC)

	res, err := newTestReport(f).DailySalesSummary(context.Background(), day)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res)
	want := `{"report_type":"daily_sales","date":"2024-11-29","data":{"total_revenue":125000.50,` +
		`"total_orders":450,"average_order_value":277.78,"top_category":"Electronics"},"generated_at":"`
	if !strings.HasPrefix(string(raw), want) || !strings.HasSuffix(string(raw), `Z"}`) {
		t.Fatalf("json mismatch:\n%s", raw)
	}
	if !f.gotDate.Equal(day) {
		t.Fatalf("repository got date %v", f.gotDate)
	}
}

func TestDailySalesSummaryEmptyDay(t *testing.T) {
	f := &fakeRepo{
		summary: entity.DailySalesSummaryEntity{TotalRevenue: "0.00", AverageOrderValue: "0.00"},
	}
	res, err := newTestReport(f).DailySalesSummary(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res.Data)
	if string(
		raw,
	) != `{"total_revenue":0.00,"total_orders":0,"average_order_value":0.00,"top_category":null}` {
		t.Fatalf("got %s", raw)
	}
}

func TestPlan(t *testing.T) {
	day := time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		p    Param
		args []any
	}{
		{Param{Type: CustomerCohort, Year: 2025}, []any{2025}},
		{Param{Type: SalesTrend, Days: 30}, []any{30}},
		{Param{Type: RFMSegmentation}, nil},
		{Param{Type: DailySalesSummary, Date: day}, []any{"2024-11-29"}},
	}
	for _, c := range cases {
		st, err := Plan(c.p)
		if err != nil {
			t.Fatalf("%s: %v", c.p.Type, err)
		}
		if len(st.Args) != len(c.args) {
			t.Fatalf("%s: args %v, want %v", c.p.Type, st.Args, c.args)
		}
		for i := range c.args {
			if st.Args[i] != c.args[i] {
				t.Fatalf("%s: args %v, want %v", c.p.Type, st.Args, c.args)
			}
		}
		if !strings.HasPrefix(st.SQL, "WITH") {
			t.Fatalf("%s: SQL should be dedented, starts with %q", c.p.Type, st.SQL[:10])
		}
		out := st.String()
		if !strings.Contains(out, "-- report: "+string(c.p.Type)) ||
			!strings.HasSuffix(out, ";\n") {
			t.Fatalf("%s: unexpected render:\n%s", c.p.Type, out)
		}
	}
	if _, err := Plan(Param{Type: SalesTrend}); err == nil {
		t.Fatal("Plan should validate params")
	}
}

func TestDedent(t *testing.T) {
	in := "\n\tSELECT\n\t\ta\n\n\tFROM t\n\t"
	if got := dedent(in); got != "SELECT\n\ta\n\nFROM t" {
		t.Fatalf("got %q", got)
	}
}
