package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func sampleSheet() Sheet {
	growth := -1.5
	var missing *float64
	return Sheet{
		Name:    "Sales Trend",
		Columns: []string{"Day", "Orders", "Revenue", "Revenue vs Avg %", "Note"},
		Rows: [][]any{
			{time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC), int64(15), 4541.9, &growth, "ok, fine"},
			{time.Date(2024, 11, 30, 0, 0, 0, 0, time.UTC), int64(0), 0.0, missing, nil},
		},
	}
}

func TestParseFormat(t *testing.T) {
	for _, in := range []string{"csv", "XLSX", " json ", "table"} {
		if _, err := ParseFormat(in); err != nil {
			t.Errorf("ParseFormat(%q): %v", in, err)
		}
	}
	if _, err := ParseFormat("pdf"); err == nil {
		t.Error("ParseFormat(pdf) should fail")
	}
}

func TestKey(t *testing.T) {
	cases := map[string]string{
		"Revenue vs Avg %":     "revenue_vs_avg_pct",
		"Top 20% in Category":  "top_20_pct_in_category",
		"Revenue 7d Avg":       "revenue_7d_avg",
		"Days Until Stock-out": "days_until_stock_out",
		"R":                    "r",
	}
	for in, want := range cases {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, CSV, sampleSheet()); err != nil {
		t.Fatal(err)
	}
	want := "Day,Orders,Revenue,Revenue vs Avg %,Note\n" +
		"2024-11-29,15,4541.90,-1.50,\"ok, fine\"\n" +
		"2024-11-30,0,0.00,,\n"
	if buf.String() != want {
		t.Fatalf("csv mismatch:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, JSON, sampleSheet()); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Report  string           `json:"report"`
		Columns []string         `json:"columns"`
		Rows    []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
	if doc.Report != "Sales Trend" || len(doc.Rows) != 2 {
		t.Fatalf("unexpected doc: %+v", doc)
	}
	first := doc.Rows[0]
	if first["day"] != "2024-11-29" || first["orders"] != 15.0 ||
		first["revenue_vs_avg_pct"] != -1.5 {
		t.Fatalf("unexpected first row: %v", first)
	}
	if v, ok := doc.Rows[1]["revenue_vs_avg_pct"]; !ok || v != nil {
		t.Fatalf("nil pointer should be null, got %v (present=%v)", v, ok)
	}
	// Keys follow column order, not alphabetical order.
	if !strings.Contains(buf.String(), `{"day": "2024-11-29", "orders": 15`) {
		t.Fatalf("keys not in column order:\n%s", buf.String())
	}
}

func TestWriteJSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, JSON, Sheet{Name: "Empty", Columns: []string{"A"}}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if rows := doc["rows"].([]any); len(rows) != 0 {
		t.Fatalf("want empty rows, got %v", rows)
	}
}

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Table, sampleSheet()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], "Revenue vs Avg %") ||
		!strings.Contains(lines[1], "4541.90") {
		t.Fatalf("unexpected table:\n%s", buf.String())
	}
}

func TestWriteXLSX(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, XLSX, sampleSheet()); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if got := f.GetSheetList(); len(got) != 1 || got[0] != "Sales Trend" {
		t.Fatalf("sheets = %v", got)
	}
	if v, _ := f.GetCellValue("Sales Trend", "A2"); v != "2024-11-29" {
		t.Errorf("date cell = %q", v)
	}
	if v, _ := f.GetCellValue(
		"Sales Trend",
		"C2",
		excelize.Options{RawCellValue: true},
	); v != "4541.9" {
		t.Errorf("revenue should be stored as a number, got %q", v)
	}
	if v, _ := f.GetCellValue("Sales Trend", "D3"); v != "" {
		t.Errorf("nil should be an empty cell, got %q", v)
	}
}

func TestWriteUnknownFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, Format("pdf"), Sheet{}); err == nil {
		t.Fatal("want error")
	}
}
