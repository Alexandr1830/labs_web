package config

import (
	"os"
	"strconv"
)

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
	Region    string
}

// getenv — возвращает значение env-переменной или fallback.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// LoadMinioConfig — читает настройки MinIO из переменных окружения.
// MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY, MINIO_BUCKET,
// MINIO_USE_SSL, MINIO_REGION. Все имеют разумные дефолты для локалки.
func LoadMinioConfig() (*MinioConfig, error) {
	useSSL, _ := strconv.ParseBool(getenv("MINIO_USE_SSL", "false"))
	return &MinioConfig{
		Endpoint:  getenv("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey: getenv("MINIO_ACCESS_KEY", "root"),
		SecretKey: getenv("MINIO_SECRET_KEY", "rootpassword"),
		Bucket:    getenv("MINIO_BUCKET", "services"),
		Region:    getenv("MINIO_REGION", "us-east-1"),
		UseSSL:    useSSL,
	}, nil
}
