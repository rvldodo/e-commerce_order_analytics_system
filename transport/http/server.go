package http

import (
	"context"
	"e-commerce_order_analytics_system/internal/config"
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/transport/http/middleware"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server struct {
	server *config.ServerConfig
	gin    *gin.Engine
	http   *http.Server
}

func New(cfg *config.Applications) *Server {
	if cfg.Server.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	limiter, rl := middleware.RateLimit(middleware.RateLimitConfig{
		Requests: 60,
		Window:   time.Minute,
	})
	defer rl.Stop()

	router.Use(middleware.GinLogging())
	router.Use(limiter)

	router.SetTrustedProxies([]string{"76.13.196.231", "127.0.0.1"})
	router.TrustedPlatform = gin.PlatformCloudflare
	router.TrustedPlatform = gin.PlatformGoogleAppEngine

	router.Use(gin.Recovery())

	return &Server{
		server: cfg.Server,
		gin:    router,
	}
}

func (s *Server) Run() error {
	handler := RegisteredRouters(s.gin)

	s.http = &http.Server{
		Addr:    s.server.Addrs,
		Handler: handler,
	}

	logger.Log.Info("Server running", zap.String("port", s.server.Addrs))
	return s.gin.Run(s.server.Addrs)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		logger.Log.Error("Error shutting down main server", zap.Error(err))
	}

	return nil
}
