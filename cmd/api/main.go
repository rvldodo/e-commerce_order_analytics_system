package main

import (
	"context"
	_ "e-commerce_order_analytics_system/cmd/docs"
	"e-commerce_order_analytics_system/internal/adapter/postgres"
	"e-commerce_order_analytics_system/internal/config"
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/pkg/jwt"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/pkg/validation"
	"e-commerce_order_analytics_system/transport/http"
	"e-commerce_order_analytics_system/transport/http/handler"
	httpnet "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// @title						Screening Test: Data Automation & Retrieval Engineer (PostgreSQL / Go)
// @version					1.0
// @description				This is a documentation for Screening Test: Data Automation & Retrieval Engineer (PostgreSQL / Go)
//
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				User API token issued by POST /api/v1/auth/login. Send it as "Bearer <token>".
func main() {
	cfg := config.New()

	if err := logger.Init(cfg.Server.Mode); err != nil {
		panic(err)
	}
	defer logger.Sync()

	// NOTE: Intialize DB
	db, err := postgres.NewDatabasePostgres(cfg.Database)
	if err != nil {
		logger.Sugar.Fatalf("Failed to initalize Database: %s", err.Error())
	}

	// NOTE: Intialize Repository
	repo := repository.New(db)

	// NOTE: Intialize JWT Tokenizer
	jwtTokenizer := jwt.New(jwt.Config{
		AccessTokenSecret:  cfg.JWT.JWTSecret,
		RefreshTokenSecret: cfg.JWT.JWTRefreshSecret,
	})

	// NOTE: Intialize Validator & Sanitizer
	validation.New()

	handler.New(repo, jwtTokenizer)

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
