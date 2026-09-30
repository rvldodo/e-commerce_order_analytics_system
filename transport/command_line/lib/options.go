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

// Read when --token / --url are not given, which keeps the token out of
// shell history, process listings and crontab lines.
const (
	TokenEnv = "REPORT_API_TOKEN"
	URLEnv   = "REPORT_API_URL"
)

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

// APIOptions are the flags shared by every command that calls the API.
type APIOptions struct {
	URL     string
	Token   string
	Retries int
	Timeout time.Duration
}

type apiFlags struct {
	url, token *string
	retries    *int
	timeout    *time.Duration
}

func addAPIFlags(fs *flag.FlagSet) apiFlags {
	return apiFlags{
		url:     fs.String("url", "", ""),
		token:   fs.String("token", "", ""),
		retries: fs.Int("retries", 3, ""),
		timeout: fs.Duration("timeout", 30*time.Second, ""),
	}
}

// resolve applies the env fallbacks and validates, so --token and --url never
// have to appear in shell history or a crontab line.
func (f apiFlags) resolve() (APIOptions, error) {
	opts := APIOptions{
		URL:     strings.TrimSpace(*f.url),
		Token:   strings.TrimSpace(*f.token),
		Retries: *f.retries,
		Timeout: *f.timeout,
	}
	if opts.URL == "" {
		opts.URL = strings.TrimSpace(os.Getenv(URLEnv))
	}
	if opts.Token == "" {
		opts.Token = strings.TrimSpace(os.Getenv(TokenEnv))
	}

	switch {
	case opts.URL == "":
		return opts, fmt.Errorf("--url is required (or set %s)", URLEnv)
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

type SendOptions struct {
	APIOptions
	File string
}

func ParseSendOptions(args []string, errOut io.Writer) (SendOptions, error) {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, USAGE) }

	api := addAPIFlags(fs)
	fileFlag := fs.String("file", "", "")

	if err := fs.Parse(args); err != nil {
		return SendOptions{}, err
	}
	if fs.NArg() > 0 {
		return SendOptions{}, fmt.Errorf(
			"unexpected argument %q (flags look like --file report.json)",
			fs.Arg(0),
		)
	}

	opts := SendOptions{File: strings.TrimSpace(*fileFlag)}
	if opts.File == "" {
		return opts, fmt.Errorf("--file is required (create one with: report export)")
	}
	var err error
	opts.APIOptions, err = api.resolve()
	return opts, err
}

type PushOptions struct {
	APIOptions
	Date    time.Time
	DryRun  bool
	NoCache bool
}

func ParsePushOptions(args []string, now time.Time, errOut io.Writer) (PushOptions, error) {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, USAGE) }

	api := addAPIFlags(fs)
	dateFlag := fs.String("date", "", "")
	dryRunFlag := fs.Bool("dry-run", false, "")
	noCacheFlag := fs.Bool("no-cache", false, "")

	if err := fs.Parse(args); err != nil {
		return PushOptions{}, err
	}
	if fs.NArg() > 0 {
		return PushOptions{}, fmt.Errorf(
			"unexpected argument %q (flags look like --date 2024-11-29)",
			fs.Arg(0),
		)
	}

	opts := PushOptions{DryRun: *dryRunFlag, NoCache: *noCacheFlag}

	var err error
	if *dateFlag == "" {
		opts.Date, err = report.Yesterday(now)
	} else {
		opts.Date, err = ParseDay("--date", *dateFlag)
	}
	if err != nil {
		return opts, err
	}

	// A dry run only prints the SQL, so it doesn't need API settings.
	if opts.DryRun {
		return opts, nil
	}
	opts.APIOptions, err = api.resolve()
	return opts, err
}
