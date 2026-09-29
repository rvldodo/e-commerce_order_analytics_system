package handler

import (
	"context"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/pkg/sender"
	"e-commerce_order_analytics_system/transport/command_line/lib"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
)

func (cli *commandHandler) CmdSend(
	ctx context.Context,
	args []string,
	out, errOut io.Writer,
) error {
	opts, err := lib.ParseSendOptions(args, errOut)
	if err != nil {
		return err
	}

	info, err := os.Stat(opts.File)
	if err != nil {
		return fmt.Errorf("read --file: %w", err)
	}
	if info.Size() > sender.MaxBodySize {
		return fmt.Errorf(
			"%s is %d bytes, maximum is %d",
			opts.File,
			info.Size(),
			sender.MaxBodySize,
		)
	}
	body, err := os.ReadFile(opts.File)
	if err != nil {
		return fmt.Errorf("read --file: %w", err)
	}
	if !json.Valid(body) {
		return fmt.Errorf("%s is not valid JSON", opts.File)
	}

	res, err := sender.Send(ctx, sender.Request{
		URL:      opts.URL,
		Token:    opts.Token,
		Body:     body,
		Attempts: opts.Retries + 1,
		Timeout:  opts.Timeout,
	})
	if err != nil {
		return fmt.Errorf("send %s to %s: %w", opts.File, opts.URL, err)
	}

	logger.WithContext(ctx).Info("report sent",
		zap.String("file", opts.File),
		zap.String("url", opts.URL),
		zap.Int("status", res.StatusCode),
		zap.Int("attempts", res.Attempts),
	)
	if res.Body != "" {
		fmt.Fprintln(out, res.Body)
	}
	return nil
}
