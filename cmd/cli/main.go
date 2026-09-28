package main

import (
	"context"
	"e-commerce_order_analytics_system/internal/adapter/postgres"
	"e-commerce_order_analytics_system/internal/config"
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/command_line/command"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.New()

	if err := logger.Init(cfg.Server.Mode); err != nil {
		panic(err)
	}
	logger.SetLevel(logger.WARN)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	newReport := func() (report.ReportInterface, func(), error) {
		db, err := postgres.NewDatabasePostgres(cfg.Database)
		if err != nil {
			return nil, nil, err
		}
		return report.New(repository.New(db)), func() { postgres.CloseDatabasePostgresql(db) }, nil
	}

	code := command.Run(ctx, newReport, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	logger.Sync()
	os.Exit(code)
}
