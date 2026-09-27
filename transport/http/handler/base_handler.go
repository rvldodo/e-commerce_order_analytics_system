package handler

import (
	"e-commerce_order_analytics_system/pkg/response"

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

// func (s *serviceHandler) Auth(next func(ctx *gin.Context)) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		ctx := c.Request.Context()
// 		accessToken := c.GetHeader("WH-Access-Token")
// 		req := dto.VerifyAccessTokenParam{
// 			AccessToken: accessToken,
// 		}
//
// 		valErrs := validation.ValidateStruct("id", &req)
// 		if valErrs != "" {
// 			response.Err(c, apperror.New(apperror.InvalidRequestParams))
// 			c.Abort()
// 			return
// 		}
//
// 		res, err := s.usecase.Auth.VerifyAccessToken(ctx, req)
// 		if err != nil {
// 			response.Err(c, apperror.Wrap(apperror.InvalidJWTTokenAuth, err))
// 			c.Abort()
// 			return
// 		}
//
// 		if !res.Valid {
// 			response.Err(c, apperror.New(apperror.UnverifiedUser))
// 			return
// 		}
//
// 		c.Set(constant.USER_INFO, res.User)
// 		next(c)
// 	}
// }
