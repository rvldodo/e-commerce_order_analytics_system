package handler

import (
	"context"
	"e-commerce_order_analytics_system/pkg/cache"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/command_line/lib"
	"fmt"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (cli *commandHandler) CmdGet(
	ctx context.Context,
	args []string,
	out, errOut io.Writer,
) error {
	opts, err := lib.ParseGetOptions(args, time.Now(), errOut)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return lib.PrintPlans(out, opts.Params)
	}
	if opts.NoCache {
		ctx = cache.WithBypass(ctx)
	}

	results := cli.uc.Report.GenerateAll(ctx, opts.Params, opts.Concurrency)

	var failed []string
	for i, res := range results {
		log := logger.WithContext(ctx).With(zap.String("report", string(res.Param.Type)))
		if res.Err != nil {
			log.Error(
				"report failed",
				zap.Duration("duration", res.Duration.Round(time.Millisecond)),
				zap.Error(res.Err),
			)
			if len(results) == 1 {
				return res.Err
			}
			failed = append(failed, fmt.Sprintf("%s (%v)", res.Param.Type, res.Err))
			continue
		}

		dest, err := lib.WriteResult(out, opts, i, res.Sheet, len(results) > 1)
		if err != nil {
			log.Error("report write failed", zap.Error(err))
			failed = append(failed, fmt.Sprintf("%s (%v)", res.Param.Type, err))
			continue
		}
		log.Info("report generated",
			zap.Int("rows", len(res.Sheet.Rows)),
			zap.Duration("duration", res.Duration.Round(time.Millisecond)),
			zap.String("output", dest),
		)
	}

	if len(failed) > 0 {
		return fmt.Errorf(
			"%d of %d reports failed: %s",
			len(failed),
			len(results),
			strings.Join(failed, "; "),
		)
	}
	return nil
}
