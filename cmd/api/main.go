package main

import (
	"context"
	"e-commerce_order_analytics_system/internal/adapter/postgres"
	"e-commerce_order_analytics_system/internal/config"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/http"
	httpnet "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	cfg := config.New()

	if err := logger.Init(cfg.Server.Mode); err != nil {
		panic(err)
	}
	defer logger.Sync()

	// NOTE: Intialize DB
	_, err := postgres.NewDatabasePostgres(cfg.Database)
	if err != nil {
		logger.Sugar.Fatalf("Failed to initalize Database: %s", err.Error())
	}

	srv := http.New(cfg)

	serverChan := make(chan error, 1)
	quitChan := make(chan os.Signal, 1)
	signal.Notify(quitChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Log.Info("Starting server...")
		if err := srv.Run(); err != nil && err != httpnet.ErrServerClosed {
			serverChan <- err
		}
	}()

	select {
	case err := <-serverChan:
		logger.Log.Error("Server error:", zap.Error(err))
	case <-quitChan:
		logger.Log.Info("Shutdown signal received")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Log.Error("Shutdown server error", zap.Error(err))
		}
		logger.Log.Info("Server shutdown completed")
	}
}
