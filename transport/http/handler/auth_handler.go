package handler

import (
	"e-commerce_order_analytics_system/pkg/response"
	"e-commerce_order_analytics_system/pkg/validation"
	"e-commerce_order_analytics_system/transport/http/dto"

	"github.com/gin-gonic/gin"
)

// Authentication godoc
//
//	@Summary		Login
//	@Description	Customer login using a registered email. Returns an access token (valid 1 day) and a refresh token (valid 30 days).
//	@Tags			Authentication
//	@Accept			application/json
//	@Produce		application/json
//	@Param			request	body		dto.LoginParam	true	"Login body request"
//	@Success		200		{object}	dto.LoginResult
//	@Failure		401		{object}	response.Problem	"Email is not registered"
//	@Failure		422		{object}	response.Problem	"Validation failed"
//	@Failure		429		{object}	response.Problem	"Too many requests"
//	@Failure		500		{object}	response.Problem	"Internal server error"
//	@Router			/api/v1/auth/login [post]
func (svc *serviceHandler) Login(ctx *gin.Context) {
	req := dto.LoginParam{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(ctx, []response.FieldError{
			{Field: "body", Message: err.Error()},
		})
		return
	}

	req.Email = validation.Sanitize(req.Email)
	valErrs := validation.ValidateStruct("en", &req)
	if valErrs != "" {
		response.ValidationFailed(ctx, []response.FieldError{
			{Field: "email", Message: valErrs},
		})
		return
	}

	res, err := svc.usecase.Auth.Login(ctx.Request.Context(), req)
	if err != nil {
		response.Err(ctx, err)
		return
	}

	response.OK(ctx, res)
}
