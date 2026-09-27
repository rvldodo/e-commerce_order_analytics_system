package http

import (
	"e-commerce_order_analytics_system/transport/http/handler"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func RegisteredRouters(r *gin.Engine) *gin.Engine {
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/reference", handler.GetHandler().ScalarReference)

	apiGroup := r.Group("/api")

	apiGroup.GET("/health-check", handler.GetHandler().HealthCheck)

	return r
}
