package repository

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"shadmin/domain"
	"shadmin/pkg"
)

type diskFileRepository struct {
	basePath string
}

func (dfr *diskFileRepository) validatePath(targetPath string) error {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("invalid path: %v", err)
	}
	absBase, err := filepath.Abs(dfr.basePath)
	if err != nil {
		return fmt.Errorf("invalid base path: %v", err)
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path traversal detected: access denied")
	}
	return nil
}

func NewDiskFileRepository(basePath string) (domain.FileRepository, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create storage base path %s: %w", basePath, err)
	}
	return &diskFileRepository{
		basePath: basePath,
	}, nil
}

func (dfr *diskFileRepository) Upload(ctx context.Context, req *domain.UploadRequest) (*domain.UploadResult, error) {
	bucketPath := filepath.Join(dfr.basePath, req.Bucket)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bucket directory: %v", err)
	}

	objectName := req.ObjectName
	if req.Prefix != "" {
		objectName = req.Prefix + "/" + req.ObjectName
		prefixPath := filepath.Join(bucketPath, req.Prefix)
		if err := os.MkdirAll(prefixPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create prefix directory: %v", err)
		}
	}

	filePath := filepath.Join(bucketPath, objectName)

	if err := dfr.validatePath(filePath); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create file directory: %v", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %v", err)
	}
	defer func() { _ = file.Close() }()

	hasher := md5.New()
	writer := io.MultiWriter(file, hasher)

	size, err := io.Copy(writer, req.Reader)
	if err != nil {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("failed to write file: %v", err)
	}

	if req.Size > 0 && size != req.Size {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("file size mismatch: expected %d, got %d", req.Size, size)
	}

	etag := fmt.Sprintf("%x", hasher.Sum(nil))

	return &domain.UploadResult{
		Bucket:     req.Bucket,
		Key:        objectName,
		ETag:       etag,
		Size:       size,
		ObjectName: objectName,
	}, nil
}

func (dfr *diskFileRepository) Download(ctx context.Context, req *domain.DownloadRequest) (io.ReadCloser, error) {
	filePath := filepath.Join(dfr.basePath, req.Bucket, req.ObjectName)

	if err := dfr.validatePath(filePath); err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", req.ObjectName)
		}
		return nil, fmt.Errorf("failed to open file: %v", err)
	}

	return file, nil
}

func (dfr *diskFileRepository) Delete(ctx context.Context, bucket, objectName string) error {
	filePath := filepath.Join(dfr.basePath, bucket, objectName)

	if err := dfr.validatePath(filePath); err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete file: %v", err)
	}

	dfr.removeEmptyDirs(filepath.Dir(filePath), filepath.Join(dfr.basePath, bucket))

	return nil
}

func (dfr *diskFileRepository) List(ctx context.Context, bucket, prefix string) ([]domain.FileInfo, error) {
	bucketPath := filepath.Join(dfr.basePath, bucket)

	if err := dfr.validatePath(bucketPath); err != nil {
		return nil, err
	}

	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		return []domain.FileInfo{}, nil
	}

	var files []domain.FileInfo
	searchPath := bucketPath
	if prefix != "" {
		searchPath = filepath.Join(bucketPath, prefix)
		if err := dfr.validatePath(searchPath); err != nil {
			return nil, err
		}
	}

	err := filepath.Walk(searchPath, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fullPath == searchPath {
			return nil
		}

		relPath, err := filepath.Rel(bucketPath, fullPath)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		if info.IsDir() && !strings.HasSuffix(relPath, "/") {
			relPath += "/"
		}

		fileType := "folder"
		if !info.IsDir() {
			fileType = pkg.GetFileType(relPath)
		}

		var etag string
		if !info.IsDir() {
			if hash, err := dfr.calculateFileHash(fullPath); err == nil {
				etag = hash
			}
		}

		fileInfo := domain.FileInfo{
			Key:          relPath,
			Name:         info.Name(),
			LastModified: info.ModTime(),
			Size:         info.Size(),
			Type:         fileType,
			Path:         relPath,
			IsFolder:     info.IsDir(),
			ETag:         etag,
		}

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %v", err)
	}

	return files, nil
}

func (dfr *diskFileRepository) GetObjectStat(ctx context.Context, bucket, objectName string) (*domain.FileInfo, error) {
	filePath := filepath.Join(dfr.basePath, bucket, objectName)

	if err := dfr.validatePath(filePath); err != nil {
		return nil, err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", objectName)
		}
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	fileType := pkg.GetFileType(objectName)
	if info.IsDir() {
		fileType = "folder"
	}

	var etag string
	if !info.IsDir() {
		if hash, err := dfr.calculateFileHash(filePath); err == nil {
			etag = hash
		}
	}

	contentType := "application/octet-stream"
	if !info.IsDir() {
		if ct := dfr.detectContentType(filePath); ct != "" {
			contentType = ct
		}
	}

	return &domain.FileInfo{
		Key:          objectName,
		Name:         path.Base(objectName),
		LastModified: info.ModTime(),
		Size:         info.Size(),
		Type:         fileType,
		Path:         objectName,
		IsFolder:     info.IsDir(),
		ETag:         etag,
		ContentType:  contentType,
	}, nil
}

func (dfr *diskFileRepository) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func (dfr *diskFileRepository) detectContentType(filePath string) string {
	if err := dfr.validatePath(filePath); err != nil {
		return ""
	}
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return ""
	}

	return pkg.DetectContentType(buffer[:n], filepath.Ext(filePath))
}

func (dfr *diskFileRepository) removeEmptyDirs(dirPath, stopAt string) {
	if dirPath == stopAt || dirPath == dfr.basePath {
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil || len(entries) > 0 {
		return
	}

	if err := os.Remove(dirPath); err == nil {
		dfr.removeEmptyDirs(filepath.Dir(dirPath), stopAt)
	}
}
