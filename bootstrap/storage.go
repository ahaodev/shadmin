package bootstrap

import (
	"fmt"
	"log/slog"
	"strings"

	"shadmin/domain"
	"shadmin/internal/conf"
	"shadmin/repository"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type StorageConfig struct {
	MinioClient *minio.Client
	FileStorage domain.FileRepository
}

func InitStorage(env *conf.Env) (*StorageConfig, error) {
	switch strings.ToLower(env.StorageType) {
	case "minio":
		return initMinioStorage(env)
	default: // "disk" 以及未指定
		return initDiskStorage(env)
	}
}

func initMinioStorage(env *conf.Env) (*StorageConfig, error) {
	minioClient, err := minio.New(env.S3Address, &minio.Options{
		Creds:  credentials.NewStaticV4(env.S3AccessKey, env.S3SecretKey, env.S3Token),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to MinIO: %w", err)
	}

	slog.Info("文件存储: 使用 MinIO 对象存储")
	return &StorageConfig{MinioClient: minioClient, FileStorage: repository.NewFileRepository(minioClient)}, nil
}

func initDiskStorage(env *conf.Env) (*StorageConfig, error) {
	fs, err := repository.NewDiskFileRepository(env.StorageBasePath)
	if err != nil {
		return nil, err
	}
	slog.Info("文件存储: 使用本地磁盘存储", "path", env.StorageBasePath)
	return &StorageConfig{FileStorage: fs}, nil
}
