package lib

import (
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/export"
	"e-commerce_order_analytics_system/pkg/sender"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TokenEnv is read when --token is not given, which keeps the token out of
// shell history and process listings.
const TokenEnv = "REPORT_API_TOKEN"

type ExportOptions struct {
	Date    time.Time
	Format  export.Format
	Out     string
	DryRun  bool
	NoCache bool
}

func ParseExportOptions(args []string, now time.Time, errOut io.Writer) (ExportOptions, error) {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, USAGE) }

	dateFlag := fs.String("date", "", "")
	formatFlag := fs.String("format", string(export.JSON), "")
	outFlag := fs.String("out", "", "")
	dryRunFlag := fs.Bool("dry-run", false, "")
	noCacheFlag := fs.Bool("no-cache", false, "")

	if err := fs.Parse(args); err != nil {
		return ExportOptions{}, err
	}
	if fs.NArg() > 0 {
		return ExportOptions{}, fmt.Errorf(
			"unexpected argument %q (flags look like --date 2024-11-29)",
			fs.Arg(0),
		)
	}

	opts := ExportOptions{Out: *outFlag, DryRun: *dryRunFlag, NoCache: *noCacheFlag}

	var err error
	if opts.Format, err = export.ParseFormat(*formatFlag); err != nil {
		return opts, err
	}
	if *dateFlag == "" {
		opts.Date, err = report.Yesterday(now)
	} else {
		opts.Date, err = ParseDay("--date", *dateFlag)
	}
	if err != nil {
		return opts, err
	}

	if opts.Out == "" && opts.Format != export.Table {
		opts.Out = filepath.Join("reports", fmt.Sprintf(
			"%s_%s.%s", report.DailySalesSummary, opts.Date.Format(time.DateOnly), opts.Format,
		))
	}

	return opts, nil
}

type SendOptions struct {
	URL     string
	Token   string
	File    string
	Retries int
	Timeout time.Duration
}

func ParseSendOptions(args []string, errOut io.Writer) (SendOptions, error) {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, USAGE) }

	urlFlag := fs.String("url", "", "")
	tokenFlag := fs.String("token", "", "")
	fileFlag := fs.String("file", "", "")
	retriesFlag := fs.Int("retries", 3, "")
	timeoutFlag := fs.Duration("timeout", 30*time.Second, "")

	if err := fs.Parse(args); err != nil {
		return SendOptions{}, err
	}
	if fs.NArg() > 0 {
		return SendOptions{}, fmt.Errorf(
			"unexpected argument %q (flags look like --file report.json)",
			fs.Arg(0),
		)
	}

	opts := SendOptions{
		URL:     strings.TrimSpace(*urlFlag),
		Token:   strings.TrimSpace(*tokenFlag),
		File:    strings.TrimSpace(*fileFlag),
		Retries: *retriesFlag,
		Timeout: *timeoutFlag,
	}
	if opts.Token == "" {
		opts.Token = strings.TrimSpace(os.Getenv(TokenEnv))
	}

	switch {
	case opts.URL == "":
		return opts, fmt.Errorf("--url is required")
	case opts.File == "":
		return opts, fmt.Errorf("--file is required (create one with: report export)")
	case opts.Token == "":
		return opts, fmt.Errorf("--token is required (or set %s)", TokenEnv)
	case opts.Retries < 0 || opts.Retries > 10:
		return opts, fmt.Errorf("--retries must be between 0 and 10")
	case opts.Timeout <= 0:
		return opts, fmt.Errorf("--timeout must be positive, e.g. 30s")
	}
	if _, err := sender.ValidateURL(opts.URL); err != nil {
		return opts, err
	}

	return opts, nil
}
