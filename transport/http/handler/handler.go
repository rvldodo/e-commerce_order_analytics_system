package handler

import (
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/internal/usecase"
	"e-commerce_order_analytics_system/pkg/jwt"
	"e-commerce_order_analytics_system/pkg/logger"

	"github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/gin-gonic/gin"
)

type serviceHandler struct {
	usecase usecase.UsecaseStruct
}

var svcHandler *serviceHandler

func New(
	repo *repository.RepoStruct,
	tokenizer *jwt.Tokenizer,
) *serviceHandler {
	svcHandler = &serviceHandler{
		usecase: *usecase.New(repo, tokenizer),
	}

	return svcHandler
}

func GetHandler() *serviceHandler {
	if svcHandler == nil {
		panic("handler.New() must be called before GetHandler()")
	}

	return svcHandler
}

func (s *serviceHandler) ScalarReference(ctx *gin.Context) {
	htmlContent, err := scalar.ApiReferenceHTML(&scalar.Options{
		// SpecURL: "https://generator3.swagger.io/openapi.json",// allow external URL or local path file
		SpecURL: "http://localhost:2002/docs/doc.json",
		CustomOptions: scalar.CustomOptions{
			PageTitle: "Screening Test: Data Automation & Retrieval Engineer (PostgreSQL / Go)",
		},
		Layout: "modern",
	})

	if err != nil {
		logger.Sugar.Infof("%v", err)
	}

	ctx.Data(200, "text/html; charset=utf-8", []byte(htmlContent))
}
