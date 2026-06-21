package config

import "os"

type Config struct {
	Port      string
	MySQLDSN  string
	RedisAddr string
	JWTSecret string
}

func LoadConfig() *Config {
	mysqlDSN := getEnv("DB_DSN", "")
	if mysqlDSN == "" {
		mysqlDSN = getEnv("MYSQL_DSN", "root:rootpassword@tcp(db:3306)/task_manager?parseTime=true")
	}
	return &Config{
		Port:      getEnv("PORT", "8080"),
		MySQLDSN:  mysqlDSN,
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
		JWTSecret: getEnv("JWT_SECRET", "super-secret-key"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
