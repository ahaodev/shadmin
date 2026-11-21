package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"shadmin/domain"
	"shadmin/internal/conf"
	"shadmin/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type StorageConfig struct {
	FileStorage domain.FileRepository
}

func InitStorage(env *conf.Env) (*StorageConfig, error) {
	switch strings.ToLower(env.StorageType) {
	case conf.StoreTypeMinio:
		return initS3Storage(env, true)
	case conf.StoreTypeS3:
		return initS3Storage(env, false)
	default:
		return initDiskStorage(env)
	}
}

func initS3Storage(env *conf.Env, minio bool) (*StorageConfig, error) {
	region := strings.TrimSpace(env.S3Region)
	var endpoint string
	if minio {
		var err error
		endpoint, err = minioEndpoint(env.S3Address, env.S3Secure)
		if err != nil {
			return nil, err
		}
		if region == "" {
			region = "us-east-1"
		}
	}

	var options []func(*config.LoadOptions) error
	if region != "" {
		options = append(options, config.WithRegion(region))
	}
	if env.S3AccessKey != "" {
		provider := credentials.NewStaticCredentialsProvider(env.S3AccessKey, env.S3SecretKey, env.S3Token)
		options = append(options, config.WithCredentialsProvider(provider))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), options...)
	if err != nil {
		return nil, fmt.Errorf("load S3 config: %w", err)
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, fmt.Errorf("set S3_REGION or configure an AWS region")
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if minio {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		}
	})
	slog.Info("文件存储: 使用 S3 对象存储", "type", env.StorageType, "region", cfg.Region)
	return &StorageConfig{FileStorage: repository.NewS3FileRepository(client, minio)}, nil
}

func minioEndpoint(address string, secure bool) (string, error) {
	scheme := "http"
	if secure {
		scheme = "https"
	}
	if !strings.Contains(address, "://") {
		address = scheme + "://" + address
	}
	endpoint, err := url.Parse(address)
	if err != nil || endpoint.Scheme != scheme || endpoint.Hostname() == "" || endpoint.User != nil ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.ForceQuery || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", fmt.Errorf("invalid S3_ADDRESS for S3_SECURE=%t", secure)
	}
	return endpoint.Scheme + "://" + endpoint.Host, nil
}

func initDiskStorage(env *conf.Env) (*StorageConfig, error) {
	fs, err := repository.NewDiskFileRepository(env.StorageBasePath)
	if err != nil {
		return nil, err
	}
	slog.Info("文件存储: 使用本地磁盘存储", "path", env.StorageBasePath)
	return &StorageConfig{FileStorage: fs}, nil
}
