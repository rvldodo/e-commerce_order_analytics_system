package main

import (
	"context"
	"e-commerce_order_analytics_system/internal/adapter/postgres"
	"e-commerce_order_analytics_system/internal/config"
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/worker"
	"e-commerce_order_analytics_system/transport/worker/cronworker"
	"e-commerce_order_analytics_system/transport/worker/job"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log := logger.New()
	log.SetLevel(logger.DEBUG)
	log.SetPrefix("[CRON] ")

	cfg := config.New()

	// NOTE: Initialize superapps database
	db, err := postgres.NewDatabasePostgres(cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to initialize main database: %s", err.Error())
	}
	defer postgres.CloseDatabasePostgresql(db)

	repo := repository.New(db)
	if repo == nil {
		logger.Fatal("Failed to create repository")
	}

	job := job.NewJob(repo)
	wk := worker.NewWorker(job)

	mgr := cronworker.New(
		cronworker.WithWorkers(8),
		cronworker.WithQueueSize(256),
		cronworker.WithLogger(log),
		cronworker.WithMiddleware(cronworker.Observe(func(job string, d time.Duration, err error) {
		})),
	)

	mgr = wk.RegisteredWorkers(mgr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := mgr.Start(ctx); err != nil {
		log.Errorf(ctx, "cron start failed: %v", err)
		os.Exit(1)
	}

	<-ctx.Done()
	stop() // restore default signal handling: a second Ctrl-C kills hard

	shutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := mgr.Stop(shutCtx); err != nil {
		log.Errorf(context.Background(), "unclean cron shutdown: %v", err)
		os.Exit(1)
	}
}
