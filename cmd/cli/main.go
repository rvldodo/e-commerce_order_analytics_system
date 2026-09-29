package main

import (
	"context"
	"e-commerce_order_analytics_system/internal/adapter/postgres"
	"e-commerce_order_analytics_system/internal/config"
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/internal/repository/postgresql"
	"e-commerce_order_analytics_system/internal/usecase/report"
	"e-commerce_order_analytics_system/pkg/cache"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/command_line/command"
	"e-commerce_order_analytics_system/transport/command_line/handler"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg := config.New()

	if err := logger.Init(cfg.Server.Mode); err != nil {
		panic(err)
	}
	logger.SetLevel(logger.INFO)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	defer logger.Sync()

	var repo *repository.RepoStruct
	if command.NeedsDatabase(os.Args[1:]) {
		db, err := postgres.NewDatabasePostgres(cfg.Database)
		if err != nil {
			logger.Errorf(ctx, "Failed to initialize main database: %s", err.Error())
			return 1
		}
		defer postgres.CloseDatabasePostgresql(db)

		repo = repository.New(db)
		loc, _ := time.LoadLocation(report.Timezone)
		repo.Postgres = postgresql.NewCached(
			repo.Postgres,
			cache.NewFileStore(cfg.Cache.Dir),
			cfg.Cache.TTL,
			loc,
		)
	}

	handler.New(repo)

	return command.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
}
