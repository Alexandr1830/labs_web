package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"lab1/internal/app/config"
)

var (
	once     sync.Once
	client   *minio.Client
	minioCfg *config.MinioConfig
	initErr  error
)

// Client returns initialized MinIO client and config.
func Client() (*minio.Client, *config.MinioConfig, error) {
	once.Do(func() {
		minioCfg, initErr = config.LoadMinioConfig()
		if initErr != nil {
			return
		}

		client, initErr = minio.New(minioCfg.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(minioCfg.AccessKey, minioCfg.SecretKey, ""),
			Secure: minioCfg.UseSSL,
			Region: minioCfg.Region,
		})
		if initErr != nil {
			return
		}

		ctx := context.Background()
		exists, err := client.BucketExists(ctx, minioCfg.Bucket)
		if err != nil {
			initErr = err
			return
		}
		if !exists {
			if err := client.MakeBucket(ctx, minioCfg.Bucket, minio.MakeBucketOptions{Region: minioCfg.Region}); err != nil {
				initErr = err
				return
			}
		}
	})
	return client, minioCfg, initErr
}

// ObjectURL builds public URL from object name.
func ObjectURL(cfg *config.MinioConfig, objectName string) string {
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, cfg.Endpoint, cfg.Bucket, objectName)
}

// ObjectNameFromURL extracts object name if URL matches bucket.
func ObjectNameFromURL(cfg *config.MinioConfig, url string) string {
	prefix := fmt.Sprintf("%s/%s/", cfg.Endpoint, cfg.Bucket)
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	if strings.HasPrefix(url, prefix) {
		return strings.TrimPrefix(url, prefix)
	}
	return ""
}
