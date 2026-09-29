package handler

import (
	"context"
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/cache"
	"e-commerce_order_analytics_system/pkg/export"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/command_line/lib"
	"encoding/json"
	"io"
	"time"

	"go.uber.org/zap"
)

func (cli *commandHandler) CmdExport(
	ctx context.Context,
	args []string,
	out, errOut io.Writer,
) error {
	opts, err := lib.ParseExportOptions(args, time.Now(), errOut)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return printPlans(out, []report.Param{{Type: report.DailySalesSummary, Date: opts.Date}})
	}
	if opts.NoCache {
		ctx = cache.WithBypass(ctx)
	}

	start := time.Now()
	summary, err := cli.uc.Report.DailySalesSummary(ctx, opts.Date)
	if err != nil {
		return err
	}

	write := func(w io.Writer) error {
		if opts.Format == export.JSON {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(summary)
		}
		return export.Write(w, opts.Format, report.SummarySheet(summary))
	}

	dest := opts.Out
	if opts.Format == export.Table || dest == "-" {
		dest = "stdout"
		err = write(out)
	} else {
		err = lib.WriteFileAtomic(dest, write)
	}
	if err != nil {
		return err
	}

	logger.WithContext(ctx).Info("daily sales summary exported",
		zap.String("date", summary.Date),
		zap.String("format", string(opts.Format)),
		zap.Duration("duration", time.Since(start).Round(time.Millisecond)),
		zap.String("output", dest),
	)
	return nil
}
