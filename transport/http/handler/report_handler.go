package handler

import (
	"e-commerce_order_analytics_system/pkg/response"

	"github.com/gin-gonic/gin"
)

// Reports godoc
//
//	@Summary		Get reports
//	@Description	Get reports based on report type in query (default: report_sales)
//	@Tags			Reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			report_type	query		string	false	"Report type to get"	default(report_sales)
//	@Success		200			{object}	dto.ReportResult
//	@Failure		400			{object}	response.Problem	"Bad Request"
//	@Failure		401			{object}	response.Problem	"Unauthorized"
//	@Failure		403			{object}	response.Problem	"Forbidden"
//	@Failure		404			{object}	response.Problem	"Data not found"
//	@Failure		429			{object}	response.Problem	"Too many requests"
//	@Failure		500			{object}	response.Problem	"Internal server error"
//	@Router			/api/v1/reports [get]
func (svc *serviceHandler) GetReports(ctx *gin.Context) {
	res, err := svc.usecase.Report.ReportDailySales(ctx.Request.Context())
	if err != nil {
		response.Err(ctx, err)
		return
	}

	response.OK(ctx, res)
}
