package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"shadmin/domain"
	"shadmin/pkg"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type s3FileRepository struct {
	client               *s3.Client
	uploader             *transfermanager.Client
	createBucketOnUpload bool
}

func NewS3FileRepository(client *s3.Client, createBucketOnUpload bool) domain.FileRepository {
	return &s3FileRepository{
		client: client,
		uploader: transfermanager.New(client, func(o *transfermanager.Options) {
			o.RequestChecksumCalculation = client.Options().RequestChecksumCalculation
			o.FailTimeout = time.Minute
		}),
		createBucketOnUpload: createBucketOnUpload,
	}
}

func (fr *s3FileRepository) Upload(ctx context.Context, req *domain.UploadRequest) (*domain.UploadResult, error) {
	objectName := req.ObjectName
	if req.Prefix != "" {
		objectName = req.Prefix + "/" + objectName
	}
	if fr.createBucketOnUpload {
		if err := fr.ensureBucket(ctx, req.Bucket); err != nil {
			return nil, err
		}
	}
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	input := &transfermanager.UploadObjectInput{
		Bucket:      aws.String(req.Bucket),
		Key:         aws.String(objectName),
		Body:        req.Reader,
		ContentType: aws.String(contentType),
	}
	if req.Size >= 0 {
		input.Body = &s3UploadReader{source: req.Reader, remaining: req.Size}
		input.ContentLength = aws.Int64(req.Size)
	}
	result, err := fr.uploader.UploadObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("upload object: %w", err)
	}
	return &domain.UploadResult{
		Bucket:     req.Bucket,
		Key:        objectName,
		ETag:       s3ETag(result.ETag),
		Size:       aws.ToInt64(result.ContentLength),
		ObjectName: objectName,
	}, nil
}

// s3UploadReader checks the declared size before the transfer manager can
// commit a single PUT or complete a multipart upload.
type s3UploadReader struct {
	source    io.Reader
	remaining int64
}

func (r *s3UploadReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		n, err := r.source.Read(p[:1])
		if n > 0 {
			return 0, errors.New("upload data exceeds declared size")
		}
		return 0, err
	}
	n, err := r.source.Read(p[:min(int64(len(p)), r.remaining)])
	r.remaining -= int64(n)
	if err == io.EOF && r.remaining > 0 {
		return n, fmt.Errorf("upload data is %d bytes shorter than declared: %w", r.remaining, io.ErrUnexpectedEOF)
	}
	return n, err
}

func (fr *s3FileRepository) ensureBucket(ctx context.Context, bucket string) error {
	_, err := fr.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	var response *smithyhttp.ResponseError
	if !errors.As(err, &response) || response.HTTPStatusCode() != http.StatusNotFound {
		return fmt.Errorf("check bucket: %w", err)
	}

	input := &s3.CreateBucketInput{Bucket: aws.String(bucket)}
	if region := fr.client.Options().Region; region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}
	_, err = fr.client.CreateBucket(ctx, input)
	var alreadyOwned *types.BucketAlreadyOwnedByYou
	if err != nil && !errors.As(err, &alreadyOwned) {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

func (fr *s3FileRepository) Download(ctx context.Context, req *domain.DownloadRequest) (io.ReadCloser, error) {
	result, err := fr.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.ObjectName),
	})
	if err != nil {
		return nil, fmt.Errorf("download object: %w", err)
	}
	return result.Body, nil
}

func (fr *s3FileRepository) Delete(ctx context.Context, bucket, objectName string) error {
	_, err := fr.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (fr *s3FileRepository) List(ctx context.Context, bucket, prefix string) ([]domain.FileInfo, error) {
	pages := s3.NewListObjectsV2Paginator(fr.client, &s3.ListObjectsV2Input{
		Bucket:       aws.String(bucket),
		Prefix:       aws.String(prefix),
		Delimiter:    aws.String("/"),
		EncodingType: types.EncodingTypeUrl,
	})
	var files []domain.FileInfo
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, object := range page.Contents {
			key, err := s3ListKey(object.Key, page.EncodingType)
			if err != nil {
				return nil, err
			}
			size := aws.ToInt64(object.Size)
			isFolder := size == 0 && strings.HasSuffix(key, "/")
			if isFolder && key == prefix {
				continue
			}
			fileType := pkg.GetFileType(key)
			if isFolder {
				fileType = "folder"
			}
			files = append(files, domain.FileInfo{
				Key:          key,
				Name:         path.Base(key),
				LastModified: aws.ToTime(object.LastModified),
				Size:         size,
				Type:         fileType,
				Path:         key,
				IsFolder:     isFolder,
				ETag:         s3ETag(object.ETag),
			})
		}
		for _, commonPrefix := range page.CommonPrefixes {
			key, err := s3ListKey(commonPrefix.Prefix, page.EncodingType)
			if err != nil {
				return nil, err
			}
			files = append(files, domain.FileInfo{
				Key: key, Name: path.Base(key), Type: "folder", Path: key, IsFolder: true,
			})
		}
	}
	return files, nil
}

func (fr *s3FileRepository) GetObjectStat(ctx context.Context, bucket, objectName string) (*domain.FileInfo, error) {
	stat, err := fr.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return &domain.FileInfo{
		Key:          objectName,
		Name:         path.Base(objectName),
		LastModified: aws.ToTime(stat.LastModified),
		Size:         aws.ToInt64(stat.ContentLength),
		Type:         pkg.GetFileType(objectName),
		Path:         objectName,
		ETag:         s3ETag(stat.ETag),
		ContentType:  aws.ToString(stat.ContentType),
	}, nil
}

func s3ListKey(value *string, encoding types.EncodingType) (string, error) {
	key := aws.ToString(value)
	if encoding != types.EncodingTypeUrl {
		return key, nil
	}
	decoded, err := url.QueryUnescape(key)
	if err != nil {
		return "", fmt.Errorf("decode object key: %w", err)
	}
	return decoded, nil
}

func s3ETag(value *string) string {
	return strings.Trim(aws.ToString(value), `"`)
}
