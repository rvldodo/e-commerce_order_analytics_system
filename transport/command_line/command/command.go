package command

import (
	"context"
	"e-commerce_order_analytics_system/transport/command_line/handler"
	"e-commerce_order_analytics_system/transport/command_line/lib"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"e-commerce_order_analytics_system/pkg/logger"

	"go.uber.org/zap"
)

func Run(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(errOut, lib.USAGE)
		return 2
	}

	command, rest := args[0], args[1:]

	start := time.Now()
	var err error
	defer func() {
		fields := []zap.Field{
			zap.String("command", command),
			zap.Duration("duration", time.Since(start).Round(time.Millisecond)),
		}
		if err != nil && !errors.Is(err, flag.ErrHelp) {
			logger.WithContext(ctx).Error("command failed", append(fields, zap.Error(err))...)
			return
		}
		logger.WithContext(ctx).Info("command finished", fields...)
	}()
	switch command {
	case "get":
		err = handler.GetHandler().CmdGet(ctx, rest, out, errOut)
	case "export":
		err = handler.GetHandler().CmdExport(ctx, rest, out, errOut)
	case "send":
		err = handler.GetHandler().CmdSend(ctx, rest, out, errOut)
	case "types":
		err = handler.GetHandler().CmdTypes(out)
	case "help", "-h", "--help":
		return handler.GetHandler().CmdHelp(out)
	default:
		err = fmt.Errorf("unknown command %q", command)
		fmt.Fprintf(errOut, "Error: %v\n\n%s", err, lib.USAGE)
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

// NeedsDatabase reports whether the command line will query Postgres, so
// help, types, send and --dry-run work without a database.
func NeedsDatabase(args []string) bool {
	if len(args) == 0 || (args[0] != "get" && args[0] != "export") {
		return false
	}
	for _, a := range args[1:] {
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if name != "dry-run" || !strings.HasPrefix(a, "-") {
			continue
		}
		if !hasValue {
			return false
		}
		if on, err := strconv.ParseBool(value); err == nil && on {
			return false
		}
	}
	return true
}
