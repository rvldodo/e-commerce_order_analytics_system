package lib

import (
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/export"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

func TestParseGetOptionsDefaults(t *testing.T) {
	opts, err := ParseGetOptions(nil, testNow, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Params) != 1 || opts.Params[0].Type != report.SalesTrend ||
		opts.Params[0].Days != 90 {
		t.Fatalf("params = %+v", opts.Params)
	}
	if opts.Format != export.XLSX || opts.Concurrency != 4 || opts.DryRun || opts.NoCache {
		t.Fatalf("opts = %+v", opts)
	}
	want := filepath.Join("reports", "sales_trend_90d_2026-09-29.xlsx")
	if len(opts.Outputs) != 1 || opts.Outputs[0] != want {
		t.Fatalf("outputs = %v, want %s", opts.Outputs, want)
	}
}

func TestParseGetOptionsMultipleTypes(t *testing.T) {
	opts, err := ParseGetOptions(
		[]string{
			"--type",
			"rfm_segmentation, customer_cohort,rfm_segmentation",
			"--year",
			"2025",
			"--format",
			"json",
			"--out",
			"out",
		},
		testNow,
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Params) != 2 || opts.Params[1].Year != 2025 {
		t.Fatalf("params = %+v (duplicates should be dropped)", opts.Params)
	}
	if opts.Outputs[0] != filepath.Join("out", "rfm_segmentation_2026-09-29.json") ||
		opts.Outputs[1] != filepath.Join("out", "customer_cohort_2025_2026-09-29.json") {
		t.Fatalf("outputs = %v", opts.Outputs)
	}
}

func TestParseGetOptionsAll(t *testing.T) {
	opts, err := ParseGetOptions([]string{"--type", "all", "--dry-run"}, testNow, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Params) != len(report.Types) || !opts.DryRun {
		t.Fatalf("got %d params", len(opts.Params))
	}
	for _, p := range opts.Params {
		if p.Type == report.DailySalesSummary && p.Date.Format(time.DateOnly) != "2026-09-28" {
			t.Fatalf("summary date defaults to yesterday, got %v", p.Date)
		}
	}
}

func TestParseGetOptionsErrors(t *testing.T) {
	cases := map[string][]string{
		"unused year":          {"--type", "rfm_segmentation", "--year", "2024"},
		"unused days":          {"--type", "customer_cohort", "--days", "30"},
		"unused date":          {"--type", "sales_trend", "--date", "2024-11-29"},
		"bad date":             {"--type", "daily_sales_summary", "--date", "29-11-2024"},
		"bad type":             {"--type", "nope"},
		"empty type":           {"--type", ","},
		"bad format":           {"--format", "pdf"},
		"bad concurrency":      {"--concurrency", "0"},
		"stdout with multiple": {"--type", "rfm_segmentation,product_performance", "--out", "-"},
		"stray argument":       {"extra"},
	}
	for name, args := range cases {
		if _, err := ParseGetOptions(args, testNow, io.Discard); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestParseGetOptionsYearUsedByOneOfSeveral(t *testing.T) {
	_, err := ParseGetOptions(
		[]string{
			"--type",
			"rfm_segmentation,customer_cohort",
			"--year",
			"2025",
			"--format",
			"table",
		},
		testNow,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("--year is used by customer_cohort, got %v", err)
	}
}

func TestParseExportOptions(t *testing.T) {
	opts, err := ParseExportOptions(nil, testNow, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Format != export.JSON || opts.Date.Format(time.DateOnly) != "2026-09-28" ||
		opts.Out != filepath.Join("reports", "daily_sales_summary_2026-09-28.json") {
		t.Fatalf("opts = %+v", opts)
	}

	opts, err = ParseExportOptions(
		[]string{"--date", "2024-11-29", "--format", "csv"},
		testNow,
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Out != filepath.Join("reports", "daily_sales_summary_2024-11-29.csv") {
		t.Fatalf("out = %s", opts.Out)
	}

	if _, err := ParseExportOptions(
		[]string{"--date", "yesterday"},
		testNow,
		io.Discard,
	); err == nil {
		t.Fatal("want date error")
	}
}

func TestParseSendOptions(t *testing.T) {
	t.Setenv(TokenEnv, "")
	t.Setenv(URLEnv, "")
	base := []string{"--url", "https://api.example.com/v1/reports", "--file", "r.json"}

	if _, err := ParseSendOptions(
		base,
		io.Discard,
	); err == nil ||
		!strings.Contains(err.Error(), TokenEnv) {
		t.Fatalf("missing token should mention %s, got %v", TokenEnv, err)
	}

	t.Setenv(TokenEnv, " from-env ")
	opts, err := ParseSendOptions(base, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Token != "from-env" || opts.Retries != 3 || opts.Timeout != 30*time.Second {
		t.Fatalf("opts = %+v", opts)
	}

	opts, err = ParseSendOptions(append(base, "--token", "flag"), io.Discard)
	if err != nil || opts.Token != "flag" {
		t.Fatalf("--token should win over env: %+v %v", opts, err)
	}

	bad := map[string][]string{
		"no url":      {"--file", "r.json"},
		"no file":     {"--url", "https://x.io"},
		"plain http":  {"--url", "http://api.example.com", "--file", "r.json"},
		"retries":     append(append([]string{}, base...), "--retries", "11"),
		"bad timeout": append(append([]string{}, base...), "--timeout", "0s"),
	}
	for name, args := range bad {
		if _, err := ParseSendOptions(args, io.Discard); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestParseDay(t *testing.T) {
	if d, err := ParseDay("--date", " 2024-11-29 "); err != nil || d.Day() != 29 {
		t.Fatalf("got %v %v", d, err)
	}
	if _, err := ParseDay("--date", "2024/11/29"); err == nil {
		t.Fatal("want error")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "r.csv")
	sheet := export.Sheet{Columns: []string{"A"}, Rows: [][]any{{"x"}}}
	if err := WriteFile(path, export.CSV, sheet); err != nil {
		t.Fatal(err)
	}

	err := WriteFileAtomic(path, func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return io.ErrUnexpectedEOF
	})
	if err == nil {
		t.Fatal("want error")
	}
	entries, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*"))
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestParsePushOptions(t *testing.T) {
	t.Setenv(TokenEnv, "")
	t.Setenv(URLEnv, "")

	if _, err := ParsePushOptions(nil, testNow, io.Discard); err == nil ||
		!strings.Contains(err.Error(), URLEnv) {
		t.Fatalf("missing url should mention %s, got %v", URLEnv, err)
	}

	t.Setenv(URLEnv, "https://api.bi-platform.com/v1/reports")
	t.Setenv(TokenEnv, "secret")
	opts, err := ParsePushOptions(nil, testNow, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.URL != "https://api.bi-platform.com/v1/reports" || opts.Token != "secret" ||
		opts.Date.Format(time.DateOnly) != "2026-09-28" || opts.Retries != 3 {
		t.Fatalf("opts = %+v", opts)
	}

	opts, err = ParsePushOptions(
		[]string{"--date", "2024-11-29", "--url", "https://other.example.com/r", "--retries", "0"},
		testNow, io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if opts.URL != "https://other.example.com/r" || opts.Retries != 0 ||
		opts.Date.Format(time.DateOnly) != "2024-11-29" {
		t.Fatalf("flags should win over env: %+v", opts)
	}

	for name, args := range map[string][]string{
		"plain http": {"--url", "http://api.bi-platform.com/v1/reports"},
		"bad date":   {"--date", "29/11/2024"},
		"stray arg":  {"now"},
	} {
		if _, err := ParsePushOptions(args, testNow, io.Discard); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestParsePushOptionsDryRunNeedsNoAPI(t *testing.T) {
	t.Setenv(TokenEnv, "")
	t.Setenv(URLEnv, "")
	opts, err := ParsePushOptions([]string{"--dry-run"}, testNow, io.Discard)
	if err != nil || !opts.DryRun {
		t.Fatalf("dry run should not require --url/--token: %+v %v", opts, err)
	}
}
