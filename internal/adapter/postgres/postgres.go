package postgres

import (
	"e-commerce_order_analytics_system/internal/config"
	"e-commerce_order_analytics_system/pkg/logger"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func quoteDSNValue(v string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + replacer.Replace(v) + "'"
}

func NewDatabasePostgres(conf *config.DatabaseConfig) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		quoteDSNValue(conf.Host),
		quoteDSNValue(conf.Port),
		quoteDSNValue(conf.User),
		quoteDSNValue(conf.Password),
		quoteDSNValue(conf.DBName),
		quoteDSNValue(conf.SSLMode),
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		logger.Log.Error("postgress error", zap.Error(err))
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	logger.Sugar.Infof("Database %s connection established", conf.DBName)
	return db, nil
}

func CloseDatabasePostgresql(db *sqlx.DB) {
	logger.Log.Info("Closing mobile database connection pool...")
	db.Close()
}
