// Package export writes tabular data to csv, xlsx or an aligned text table.
package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

type Format string

const (
	CSV   Format = "csv"
	XLSX  Format = "xlsx"
	Table Format = "table"
)

func ParseFormat(s string) (Format, error) {
	switch f := Format(strings.ToLower(strings.TrimSpace(s))); f {
	case CSV, XLSX, Table:
		return f, nil
	}
	return "", fmt.Errorf("unknown format %q: expected xlsx, csv or table", s)
}

// Sheet is a titled grid. Cells may be string, integers, float64, time.Time,
// a nil pointer or nil; nil renders as an empty cell.
type Sheet struct {
	Name    string
	Columns []string
	Rows    [][]any
}

func Write(w io.Writer, f Format, s Sheet) error {
	switch f {
	case CSV:
		return writeCSV(w, s)
	case XLSX:
		return writeXLSX(w, s)
	case Table:
		return writeTable(w, s)
	}
	return fmt.Errorf("unsupported format %q", f)
}

func writeCSV(w io.Writer, s Sheet) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(s.Columns); err != nil {
		return err
	}
	record := make([]string, len(s.Columns))
	for _, row := range s.Rows {
		for i, v := range row {
			record[i] = formatText(v)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeTable(w io.Writer, s Sheet) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, strings.Join(s.Columns, "\t")+"\t")
	for _, row := range s.Rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = formatText(v)
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t")+"\t")
	}
	return tw.Flush()
}

func writeXLSX(w io.Writer, s Sheet) error {
	f := excelize.NewFile()
	defer f.Close()

	name := s.Name
	if name == "" {
		name = "Report"
	}
	if err := f.SetSheetName("Sheet1", name); err != nil {
		return err
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E7E6E6"}},
	})
	if err != nil {
		return err
	}
	decimalStyle, err := f.NewStyle(&excelize.Style{NumFmt: 4}) // #,##0.00
	if err != nil {
		return err
	}
	intStyle, err := f.NewStyle(&excelize.Style{NumFmt: 3}) // #,##0
	if err != nil {
		return err
	}
	dateFormat := "yyyy-mm-dd"
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFormat})
	if err != nil {
		return err
	}

	widths := make([]int, len(s.Columns))
	for i, col := range s.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(name, cell, col); err != nil {
			return err
		}
		widths[i] = utf8.RuneCountInString(col)
	}
	lastHeader, _ := excelize.CoordinatesToCellName(len(s.Columns), 1)
	if err := f.SetCellStyle(name, "A1", lastHeader, headerStyle); err != nil {
		return err
	}

	for r, row := range s.Rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			v = deref(v)
			if v == nil {
				continue
			}
			if err := f.SetCellValue(name, cell, v); err != nil {
				return err
			}

			style := 0
			switch v.(type) {
			case float64:
				style = decimalStyle
			case int, int64:
				style = intStyle
			case time.Time:
				style = dateStyle
			}
			if style != 0 {
				if err := f.SetCellStyle(name, cell, cell, style); err != nil {
					return err
				}
			}
			if n := utf8.RuneCountInString(formatText(v)); n > widths[c] {
				widths[c] = n
			}
		}
	}

	for i, width := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(name, col, col, float64(min(width, 60)+2)); err != nil {
			return err
		}
	}

	if len(s.Columns) > 0 {
		lastCell, _ := excelize.CoordinatesToCellName(len(s.Columns), len(s.Rows)+1)
		if err := f.AutoFilter(name, "A1:"+lastCell, nil); err != nil {
			return err
		}
		if err := f.SetPanes(name, &excelize.Panes{
			Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
		}); err != nil {
			return err
		}
	}

	_, err = f.WriteTo(w)
	return err
}

func deref(v any) any {
	switch t := v.(type) {
	case *float64:
		if t == nil {
			return nil
		}
		return *t
	case *int64:
		if t == nil {
			return nil
		}
		return *t
	case *string:
		if t == nil {
			return nil
		}
		return *t
	}
	return v
}

func formatText(v any) string {
	switch t := deref(v).(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', 2, 64)
	case time.Time:
		return t.Format(time.DateOnly)
	default:
		return fmt.Sprint(t)
	}
}
