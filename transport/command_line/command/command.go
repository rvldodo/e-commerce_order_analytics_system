package command

import (
	"context"
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/export"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"
)

const usage = `report - export e-commerce order analytics reports from PostgreSQL

Usage:
  report get [flags]    Generate a report (database connection is read from .env)
  report types          List available report types and the flags they use
  report help           Show this help

Flags for get:
  --type    Report type (default: sales_trend, see "report types")
  --year    Year for customer_cohort and purchase_patterns (default: 2024)
  --days    Window in days for sales_trend and inventory_turnover (default: 90)
  --format  xlsx, csv or table (default: xlsx; table prints to the terminal)
  --out     Output file, "-" for stdout (default: reports/<type>[_<year|days>]_<today>.<format>)
  --limit   Keep only the first N rows, 0 = all (default: 0)

Days and months are evaluated in Asia/Jakarta. Only completed orders count.

Examples:
  report get
  report get --type customer_cohort --year 2025
  report get --type rfm_segmentation --format csv --out - | head
  report get --type product_performance --format table --limit 20
  report get --type sales_trend --days 30
`

// ReportFactory opens the resources a report needs (the database) only when a
// command actually uses them, so "help" and "types" work without a database.
type ReportFactory func() (uc report.ReportInterface, closeFn func(), err error)

func Run(ctx context.Context, newReport ReportFactory, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(errOut, usage)
		return 2
	}

	command, rest := args[0], args[1:]

	var err error
	switch command {
	case "get":
		err = cmdGet(ctx, newReport, rest, out, errOut)
	case "types":
		err = cmdTypes(out)
	case "help", "-h", "--help":
		fmt.Fprint(out, usage)
		return 0
	default:
		fmt.Fprintf(errOut, "Error: unknown command %q\n\n%s", command, usage)
		return 2
	}

	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(errOut, "Error: %v\n", err)
		return 1
	}
	return 0
}

type getOptions struct {
	param  report.Param
	format export.Format
	out    string
}

func parseGetOptions(args []string, now time.Time, errOut io.Writer) (getOptions, error) {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, usage) }

	typeFlag := fs.String("type", string(report.SalesTrend), "")
	yearFlag := fs.Int("year", report.DefaultYear, "")
	daysFlag := fs.Int("days", report.DefaultDays, "")
	formatFlag := fs.String("format", string(export.XLSX), "")
	outFlag := fs.String("out", "", "")
	limitFlag := fs.Int("limit", 0, "")

	if err := fs.Parse(args); err != nil {
		return getOptions{}, err
	}
	if fs.NArg() > 0 {
		return getOptions{}, fmt.Errorf(
			"unexpected argument %q (flags look like --type sales_trend)",
			fs.Arg(0),
		)
	}

	opts := getOptions{out: *outFlag}

	var err error
	if opts.param.Type, err = report.ParseType(*typeFlag); err != nil {
		return opts, err
	}
	if opts.format, err = export.ParseFormat(*formatFlag); err != nil {
		return opts, err
	}

	// Reject flags the chosen report would silently ignore.
	info, _ := report.Lookup(opts.param.Type)
	var unused error
	fs.Visit(func(f *flag.Flag) {
		if (f.Name == "year" && !info.UsesYear) || (f.Name == "days" && !info.UsesDays) {
			unused = fmt.Errorf("--%s is not used by %s (see \"report types\")", f.Name, info.Type)
		}
	})
	if unused != nil {
		return opts, unused
	}

	opts.param.Year = *yearFlag
	opts.param.Days = *daysFlag
	opts.param.Limit = *limitFlag
	if err := opts.param.Validate(); err != nil {
		return opts, err
	}

	if opts.out == "" && opts.format != export.Table {
		name := string(opts.param.Type)
		if tag := opts.param.Tag(); tag != "" {
			name += "_" + tag
		}
		name += "_" + now.Format(time.DateOnly) + "." + string(opts.format)
		opts.out = filepath.Join("reports", name)
	}

	return opts, nil
}

func cmdGet(
	ctx context.Context,
	newReport ReportFactory,
	args []string,
	out, errOut io.Writer,
) error {
	opts, err := parseGetOptions(args, time.Now(), errOut)
	if err != nil {
		return err
	}

	uc, closeFn, err := newReport()
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer closeFn()

	start := time.Now()
	sheet, err := uc.Generate(ctx, opts.param)
	if err != nil {
		return err
	}

	if opts.format == export.Table || opts.out == "-" {
		return export.Write(out, opts.format, sheet)
	}

	if err := writeFile(opts.out, opts.format, sheet); err != nil {
		return err
	}

	// Status goes to stderr so stdout stays clean for piping.
	fmt.Fprintf(errOut, "%s report: %d rows written to %s (%s)\n",
		opts.param.Type,
		len(sheet.Rows),
		opts.out,
		time.Since(start).Round(time.Millisecond),
	)
	return nil
}

// writeFile writes to a temp file first so a failed export never leaves a
// truncated report behind.
func writeFile(path string, format export.Format, sheet export.Sheet) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".report-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := export.Write(tmp, format, sheet); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", format, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	// CreateTemp uses 0600; reports are meant to be shared.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}

func cmdTypes(out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tFLAGS\tDESCRIPTION")
	for _, t := range report.Types {
		flags := "-"
		switch {
		case t.UsesYear:
			flags = "--year"
		case t.UsesDays:
			flags = "--days"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", t.Type, flags, t.Description)
	}
	return w.Flush()
}
