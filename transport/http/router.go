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
	v1Group := apiGroup.Group("/v1")

	apiGroup.GET("/health-check", handler.GetHandler().HealthCheck)

	authGroup := v1Group.Group("/auth")
	authGroup.POST("/login", handler.GetHandler().Login)

	reportGroup := v1Group.Group("reports")
	reportGroup.GET("/", handler.GetHandler().Auth(handler.GetHandler().GetReports))

	return r
}
