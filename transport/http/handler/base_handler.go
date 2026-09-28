package handler

import (
	"strings"

	apperror "e-commerce_order_analytics_system/pkg/errors"
	"e-commerce_order_analytics_system/pkg/response"
	"e-commerce_order_analytics_system/pkg/validation"
	"e-commerce_order_analytics_system/transport/http/dto"

	"github.com/gin-gonic/gin"
)

// HealthCheck godoc
//
//	@Summary	Check the API health
//	@Tags		Health
//	@Produce	application/json
//	@Success	200
//	@Router		/api/health-check [get]
func (s *serviceHandler) HealthCheck(ctx *gin.Context) {
	response.OK(ctx, map[string]interface{}{
		"message": "Health Check OK!!!!",
	})
}

func (s *serviceHandler) Auth(next func(ctx *gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		accessToken, _ := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		req := dto.VerifyAccessTokenParam{
			AccessToken: strings.TrimSpace(accessToken),
		}

		valErrs := validation.ValidateStruct("en", &req)
		if valErrs != "" {
			response.Err(c, apperror.New(apperror.InvalidJWTTokenAuth))
			c.Abort()
			return
		}

		res, err := s.usecase.Auth.VerifyAccessToken(ctx, req)
		if err != nil {
			response.Err(c, apperror.Wrap(apperror.InvalidJWTTokenAuth, err))
			c.Abort()
			return
		}

		if !res.Valid {
			response.Err(c, apperror.New(apperror.UnverifiedUser))
			c.Abort()
			return
		}
		next(c)
	}
}
