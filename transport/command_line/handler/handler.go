package handler

import (
	"e-commerce_order_analytics_system/internal/repository"
	"e-commerce_order_analytics_system/internal/usecase"
)

type commandHandler struct {
	uc usecase.UsecaseStruct
}

var cliHandler *commandHandler

func New(repo *repository.RepoStruct) *commandHandler {
	cliHandler = &commandHandler{
		uc: *usecase.New(repo),
	}

	return cliHandler
}

func GetHandler() *commandHandler {
	if cliHandler == nil {
		panic("handler.New() must be called before GetHandler()")
	}

	return cliHandler
}
