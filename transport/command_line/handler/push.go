package handler

import (
	"context"
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/cache"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/pkg/sender"
	"e-commerce_order_analytics_system/transport/command_line/lib"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
)

// CmdPush generates the daily sales summary and posts it to the BI API in
// one step, so it can be scheduled as a single command.
func (cli *commandHandler) CmdPush(
	ctx context.Context,
	args []string,
	out, errOut io.Writer,
) error {
	opts, err := lib.ParsePushOptions(args, time.Now(), errOut)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return lib.PrintPlans(
			out,
			[]report.Param{{Type: report.DailySalesSummary, Date: opts.Date}},
		)
	}
	if opts.NoCache {
		ctx = cache.WithBypass(ctx)
	}

	summary, err := cli.uc.Report.DailySalesSummary(ctx, opts.Date)
	if err != nil {
		return err
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	res, err := sender.Send(ctx, sender.Request{
		URL:      opts.URL,
		Token:    opts.Token,
		Body:     body,
		Attempts: opts.Retries + 1,
		Timeout:  opts.Timeout,
	})
	if err != nil {
		return fmt.Errorf("push daily sales summary for %s to %s: %w", summary.Date, opts.URL, err)
	}

	logger.WithContext(ctx).Info("daily sales summary pushed",
		zap.String("date", summary.Date),
		zap.Int64("total_orders", summary.Data.TotalOrders),
		zap.String("total_revenue", summary.Data.TotalRevenue.String()),
		zap.String("url", opts.URL),
		zap.Int("status", res.StatusCode),
		zap.Int("attempts", res.Attempts),
	)
	if res.Body != "" {
		fmt.Fprintln(out, res.Body)
	}
	return nil
}
