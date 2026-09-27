package config

import "e-commerce_order_analytics_system/pkg/env"

type Applications struct {
	Server   *ServerConfig
	Database *DatabaseConfig
	JWT      *JWTConfig
}

type ServerConfig struct {
	Addrs   string
	Mode    string
	GinMode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type JWTConfig struct {
	JWTSecret        string
	JWTRefreshSecret string
}

func New() *Applications {
	return &Applications{
		Server: &ServerConfig{
			Addrs:   env.GetString("ADDRS", ":2002"),
			Mode:    env.GetString("SRV_MODE", "development"),
			GinMode: env.GetString("GIN_MODE", "debug"),
		},
		Database: &DatabaseConfig{
			Host:     env.GetString("DB_HOST", ""),
			Port:     env.GetString("DB_PORT", ""),
			User:     env.GetString("DB_USER", ""),
			Password: env.GetString("DB_PASSWORD", ""),
			DBName:   env.GetString("DB_NAME", ""),
			SSLMode:  env.GetString("DB_SSL_MODE", ""),
		},
		JWT: &JWTConfig{
			JWTSecret:        env.GetString("JWT_SECRET", "secret"),
			JWTRefreshSecret: env.GetString("JWT_REFRESH_SECRET", "refresh"),
		},
	}
}
