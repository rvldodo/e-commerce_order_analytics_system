package postgresql

import (
	"github.com/jmoiron/sqlx"
)

type pgStruct struct {
	db *sqlx.DB
}

type PostgresInterface interface {
}

func NewPG(db *sqlx.DB) PostgresInterface {
	return &pgStruct{
		db: db,
	}
}
