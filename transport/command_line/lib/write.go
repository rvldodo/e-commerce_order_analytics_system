package lib

import (
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/export"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type GetOptions struct {
	Params      []report.Param
	Outputs     []string
	Format      export.Format
	Out         string
	DryRun      bool
	NoCache     bool
	Concurrency int
}

func WriteFile(path string, format export.Format, sheet export.Sheet) error {
	return WriteFileAtomic(path, func(w io.Writer) error {
		if err := export.Write(w, format, sheet); err != nil {
			return fmt.Errorf("write %s: %w", format, err)
		}
		return nil
	})
}

// WriteFileAtomic writes to a temp file next to path and renames it into
// place, so a failed export never leaves a truncated file behind.
func WriteFileAtomic(path string, write func(w io.Writer) error) error {
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

	if err := write(tmp); err != nil {
		tmp.Close()
		return err
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

// ParseDay parses a YYYY-MM-DD flag value.
func ParseDay(name, value string) (time.Time, error) {
	day, err := time.Parse(time.DateOnly, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %q: expected YYYY-MM-DD", name, value)
	}
	return day, nil
}

func ParseGetOptions(args []string, now time.Time, errOut io.Writer) (GetOptions, error) {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, USAGE) }

	typeFlag := fs.String("type", string(report.SalesTrend), "")
	yearFlag := fs.Int("year", report.DefaultYear, "")
	daysFlag := fs.Int("days", report.DefaultDays, "")
	formatFlag := fs.String("format", string(export.XLSX), "")
	outFlag := fs.String("out", "", "")
	dateFlag := fs.String("date", "", "")
	limitFlag := fs.Int("limit", 0, "")
	dryRunFlag := fs.Bool("dry-run", false, "")
	noCacheFlag := fs.Bool("no-cache", false, "")
	concurrencyFlag := fs.Int("concurrency", 4, "")

	if err := fs.Parse(args); err != nil {
		return GetOptions{}, err
	}
	if fs.NArg() > 0 {
		return GetOptions{}, fmt.Errorf(
			"unexpected argument %q (flags look like --type sales_trend)",
			fs.Arg(0),
		)
	}

	opts := GetOptions{
		Out:         *outFlag,
		DryRun:      *dryRunFlag,
		NoCache:     *noCacheFlag,
		Concurrency: *concurrencyFlag,
	}
	if opts.Concurrency < 1 || opts.Concurrency > report.MaxConcurrency {
		return opts, fmt.Errorf("--concurrency must be between 1 and %d", report.MaxConcurrency)
	}

	types, err := ParseTypes(*typeFlag)
	if err != nil {
		return opts, err
	}
	if opts.Format, err = export.ParseFormat(*formatFlag); err != nil {
		return opts, err
	}

	var usesYear, usesDays, usesDate bool
	for _, t := range types {
		info, _ := report.Lookup(t)
		usesYear = usesYear || info.UsesYear
		usesDays = usesDays || info.UsesDays
		usesDate = usesDate || info.UsesDate
	}
	var unused error
	fs.Visit(func(f *flag.Flag) {
		if (f.Name == "year" && !usesYear) ||
			(f.Name == "days" && !usesDays) ||
			(f.Name == "date" && !usesDate) {
			unused = fmt.Errorf("--%s is not used by %s (see \"report types\")", f.Name, *typeFlag)
		}
	})
	if unused != nil {
		return opts, unused
	}

	var date time.Time
	if usesDate {
		if *dateFlag == "" {
			date, err = report.Yesterday(now)
		} else {
			date, err = ParseDay("--date", *dateFlag)
		}
		if err != nil {
			return opts, err
		}
	}

	for _, t := range types {
		p := report.Param{Type: t, Year: *yearFlag, Days: *daysFlag, Limit: *limitFlag}
		if info, _ := report.Lookup(t); info.UsesDate {
			p.Date = date
		}
		if err := p.Validate(); err != nil {
			return opts, err
		}
		opts.Params = append(opts.Params, p)
	}

	if opts.DryRun || opts.Format == export.Table {
		return opts, nil
	}

	if len(opts.Params) == 1 {
		out := opts.Out
		if out == "" {
			out = filepath.Join("reports", DefaultFileName(opts.Params[0], opts.Format, now))
		}
		opts.Outputs = []string{out}
		return opts, nil
	}

	if opts.Out == "-" {
		return opts, fmt.Errorf("--out - needs a single report; with several, --out is a directory")
	}
	dir := opts.Out
	if dir == "" {
		dir = "reports"
	}
	for _, p := range opts.Params {
		opts.Outputs = append(
			opts.Outputs,
			filepath.Join(dir, DefaultFileName(p, opts.Format, now)),
		)
	}
	return opts, nil
}

// ParseTypes accepts one type, a comma-separated list or "all".
func ParseTypes(value string) ([]report.Type, error) {
	if strings.TrimSpace(strings.ToLower(value)) == "all" {
		types := make([]report.Type, len(report.Types))
		for i, info := range report.Types {
			types[i] = info.Type
		}
		return types, nil
	}

	var types []report.Type
	seen := map[report.Type]bool{}
	for _, part := range strings.Split(value, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		t, err := report.ParseType(part)
		if err != nil {
			return nil, err
		}
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("--type is empty")
	}
	return types, nil
}

func DefaultFileName(p report.Param, format export.Format, now time.Time) string {
	name := string(p.Type)
	if tag := p.Tag(); tag != "" {
		name += "_" + tag
	}
	return name + "_" + now.Format(time.DateOnly) + "." + string(format)
}

func WriteResult(
	out io.Writer,
	opts GetOptions,
	i int,
	sheet export.Sheet,
	multiple bool,
) (string, error) {
	if opts.Format == export.Table {
		if multiple {
			fmt.Fprintf(out, "\n== %s ==\n", sheet.Name)
		}
		return "stdout", export.Write(out, opts.Format, sheet)
	}

	dest := opts.Outputs[i]
	if dest == "-" {
		return "stdout", export.Write(out, opts.Format, sheet)
	}
	return dest, WriteFile(dest, opts.Format, sheet)
}

func PrintPlans(out io.Writer, params []report.Param) error {
	for i, p := range params {
		st, err := report.Plan(p)
		if err != nil {
			return err
		}
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprint(out, st.String())
	}
	return nil
}
