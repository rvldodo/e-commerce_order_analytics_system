package repository

import (
	"e-commerce_order_analytics_system/internal/repository/postgresql"

	"github.com/jmoiron/sqlx"
)

type RepoStruct struct {
	Postgres postgresql.PostgresInterface
}

func New(db *sqlx.DB) *RepoStruct {
	return &RepoStruct{
		Postgres: postgresql.NewPG(db),
	}
}
