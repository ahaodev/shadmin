package repository

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"shadmin/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func testS3Repository(t *testing.T, handler http.HandlerFunc, createBucket bool) domain.FileRepository {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := s3.New(s3.Options{
		BaseEndpoint:               aws.String(server.URL),
		UsePathStyle:               true,
		Region:                     "us-east-1",
		Credentials:                credentials.NewStaticCredentialsProvider("test", "test", ""),
		Retryer:                    aws.NopRetryer{},
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
	})
	return NewS3FileRepository(client, createBucket)
}

// s3ServerTestClient returns a client for an S3-compatible server, skipping the
// test unless SHADMIN_TEST_S3_ENDPOINT is set. Credentials default to the
// conventional LocalStack test values and can be overridden with
// SHADMIN_TEST_S3_ACCESS_KEY and SHADMIN_TEST_S3_SECRET_KEY. The endpoint may
// omit the scheme; the test then uses plain HTTP.
func s3ServerTestClient(t *testing.T) *s3.Client {
	t.Helper()
	endpoint := os.Getenv("SHADMIN_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set SHADMIN_TEST_S3_ENDPOINT (e.g. http://localhost:4566 for LocalStack) to test against an S3-compatible server")
	}
	accessKey := cmp.Or(os.Getenv("SHADMIN_TEST_S3_ACCESS_KEY"), "test")
	secretKey := cmp.Or(os.Getenv("SHADMIN_TEST_S3_SECRET_KEY"), "test")
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	return s3.New(s3.Options{
		BaseEndpoint:               aws.String(endpoint),
		Region:                     "us-east-1",
		UsePathStyle:               true,
		Credentials:                credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
}

// s3ServerTestBucket returns a fresh bucket name and removes the bucket and
// every object left in it when the test finishes.
func s3ServerTestBucket(t *testing.T, client *s3.Client) string {
	t.Helper()
	bucket := fmt.Sprintf("shadmin-test-%x", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx := context.Background()
		_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
		if err != nil {
			return // the test never created the bucket
		}
		pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				t.Errorf("list test bucket %s for cleanup: %v", bucket, err)
				return
			}
			for _, object := range page.Contents {
				if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: object.Key}); err != nil {
					t.Errorf("delete %s during cleanup: %v", aws.ToString(object.Key), err)
				}
			}
		}
		if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			t.Errorf("delete test bucket %s: %v", bucket, err)
		}
	})
	return bucket
}

func TestS3ServerFileOperations(t *testing.T) {
	client := s3ServerTestClient(t)
	repo := NewS3FileRepository(client, true)
	bucket := s3ServerTestBucket(t, client)
	ctx := context.Background()

	const key = "prefix/a+b %.txt"
	upload, err := repo.Upload(ctx, &domain.UploadRequest{
		Bucket: bucket, Prefix: "prefix", ObjectName: "a+b %.txt",
		Reader: strings.NewReader("hello"), Size: 5, ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if upload.Bucket != bucket || upload.Key != key || upload.ObjectName != key || upload.Size != 5 || upload.ETag == "" {
		t.Fatalf("unexpected upload result: %+v", upload)
	}
	// Uploading into the now existing bucket exercises the skipped creation path.
	if _, err := repo.Upload(ctx, &domain.UploadRequest{
		Bucket: bucket, Prefix: "prefix/folder", ObjectName: "child.txt",
		Reader: strings.NewReader("child"), Size: 5,
	}); err != nil {
		t.Fatalf("upload nested object: %v", err)
	}

	files, err := repo.List(ctx, bucket, "prefix/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	listed := make(map[string]domain.FileInfo, len(files))
	for _, file := range files {
		listed[file.Key] = file
	}
	if file, ok := listed[key]; len(files) != 2 || !ok || file.Name != "a+b %.txt" || file.Size != 5 || file.IsFolder || file.Type == "folder" {
		t.Fatalf("unexpected list: %+v", files)
	}
	if folder, ok := listed["prefix/folder/"]; !ok || !folder.IsFolder || folder.Name != "folder" || folder.Type != "folder" {
		t.Fatalf("unexpected folder entry: %+v", files)
	}

	stat, err := repo.GetObjectStat(ctx, bucket, key)
	if err != nil || stat.Key != key || stat.Size != 5 || stat.ContentType != "text/plain" || stat.ETag != upload.ETag || stat.LastModified.IsZero() {
		t.Fatalf("stat = %+v, %v", stat, err)
	}

	body, err := repo.Download(ctx, &domain.DownloadRequest{Bucket: bucket, ObjectName: key})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	payload, readErr := io.ReadAll(body)
	closeErr := body.Close()
	if readErr != nil || closeErr != nil || string(payload) != "hello" {
		t.Fatalf("download = %q, read error %v, close error %v", payload, readErr, closeErr)
	}

	if err := repo.Delete(ctx, bucket, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Download(ctx, &domain.DownloadRequest{Bucket: bucket, ObjectName: key}); err == nil {
		t.Fatal("expected deleted object download to fail")
	}
	if _, err := repo.GetObjectStat(ctx, bucket, key); err == nil {
		t.Fatal("expected deleted object stat to fail")
	}
	if files, err := repo.List(ctx, bucket, "prefix/"); err != nil || len(files) != 1 || files[0].Key != "prefix/folder/" {
		t.Fatalf("list after delete = %+v, %v", files, err)
	}
	if _, err := repo.Download(ctx, &domain.DownloadRequest{Bucket: bucket, ObjectName: "missing"}); err == nil {
		t.Fatal("expected missing object to fail")
	}
}

func TestS3ServerMultipartUpload(t *testing.T) {
	client := s3ServerTestClient(t)
	repo := NewS3FileRepository(client, true)
	bucket := s3ServerTestBucket(t, client)
	ctx := context.Background()

	const size = 17 << 20 // Larger than the transfer manager's multipart threshold.
	upload, err := repo.Upload(ctx, &domain.UploadRequest{
		Bucket: bucket, ObjectName: "multipart.bin", Size: -1,
		Reader: io.LimitReader(strings.NewReader(strings.Repeat("x", size)), size),
	})
	if err != nil {
		t.Fatalf("multipart upload: %v", err)
	}
	if upload.Key != "multipart.bin" || upload.Size != size || !strings.Contains(upload.ETag, "-") {
		t.Fatalf("unexpected multipart upload result: %+v", upload)
	}
	stat, err := repo.GetObjectStat(ctx, bucket, "multipart.bin")
	if err != nil || stat.Size != size || stat.ContentType != "application/octet-stream" {
		t.Fatalf("multipart stat = %+v, %v", stat, err)
	}
}

func TestS3FileOperations(t *testing.T) {
	const key = "prefix/a+b %.txt"
	repo := testS3Repository(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bucket":
			query := r.URL.Query()
			if query.Get("list-type") != "2" || query.Get("prefix") != "prefix/" || query.Get("delimiter") != "/" || query.Get("encoding-type") != "url" {
				t.Errorf("unexpected list query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/xml")
			switch query.Get("continuation-token") {
			case "":
				fmt.Fprint(w, `<ListBucketResult><EncodingType>url</EncodingType><IsTruncated>true</IsTruncated>
					<NextContinuationToken>page+2</NextContinuationToken>
					<Contents><Key>prefix%2F</Key><Size>0</Size></Contents>
					<Contents><Key>prefix%2Fa%2Bb+%25.txt</Key><ETag>&quot;tag&quot;</ETag><Size>5</Size></Contents>
					<CommonPrefixes><Prefix>prefix%2Ffolder%2F</Prefix></CommonPrefixes></ListBucketResult>`)
			case "page+2":
				fmt.Fprint(w, `<ListBucketResult><EncodingType>url</EncodingType><IsTruncated>false</IsTruncated>
					<Contents><Key>prefix%2Fempty%2F</Key><Size>0</Size></Contents></ListBucketResult>`)
			default:
				t.Errorf("unexpected continuation token: %q", query.Get("continuation-token"))
				w.WriteHeader(http.StatusBadRequest)
			}
		case "/bucket/" + key:
			switch r.Method {
			case http.MethodGet, http.MethodHead:
				w.Header().Set("Content-Length", "5")
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("ETag", `"tag"`)
				w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
				if r.Method == http.MethodGet {
					fmt.Fprint(w, "hello")
				}
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected object method %s", r.Method)
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/bucket/missing":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `<Error><Code>NoSuchKey</Code></Error>`)
		default:
			t.Errorf("unexpected S3 path %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}, false)

	ctx := context.Background()
	files, err := repo.List(ctx, "bucket", "prefix/")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || files[0].Key != key || files[0].ETag != "tag" || files[0].Size != 5 ||
		files[1].Key != "prefix/folder/" || files[1].Name != "folder" || !files[1].IsFolder ||
		files[2].Key != "prefix/empty/" || !files[2].IsFolder {
		t.Fatalf("unexpected files: %+v", files)
	}
	body, err := repo.Download(ctx, &domain.DownloadRequest{Bucket: "bucket", ObjectName: key})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(body)
	if closeErr := body.Close(); err != nil || closeErr != nil || string(payload) != "hello" {
		t.Fatalf("download = %q, read error %v, close error %v", payload, err, closeErr)
	}
	stat, err := repo.GetObjectStat(ctx, "bucket", key)
	if err != nil || stat.Key != key || stat.Size != 5 || stat.ContentType != "text/plain" ||
		stat.ETag != "tag" || stat.LastModified.IsZero() {
		t.Fatalf("stat = %+v, %v", stat, err)
	}
	if err := repo.Delete(ctx, "bucket", key); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Download(ctx, &domain.DownloadRequest{Bucket: "bucket", ObjectName: "missing"}); err == nil {
		t.Fatal("expected missing object to fail")
	}
}

func TestS3MinioBucketCreation(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		headStatus, createStatus int
		wantUpload               bool
	}{
		{"creates missing bucket", http.StatusNotFound, http.StatusOK, true},
		{"bucket created concurrently", http.StatusNotFound, http.StatusConflict, true},
		{"access denied", http.StatusForbidden, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testS3Repository(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "HEAD /bucket":
					w.WriteHeader(tc.headStatus)
				case "PUT /bucket":
					if tc.createStatus == 0 {
						t.Error("unexpected bucket creation")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.WriteHeader(tc.createStatus)
					if tc.createStatus == http.StatusConflict {
						fmt.Fprint(w, `<Error><Code>BucketAlreadyOwnedByYou</Code></Error>`)
					}
				case "PUT /bucket/object":
					if !tc.wantUpload {
						t.Error("unexpected upload")
					}
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
				}
			}, true)
			result, err := repo.Upload(context.Background(), &domain.UploadRequest{
				Bucket: "bucket", ObjectName: "object", Reader: strings.NewReader("hello"), Size: 5,
			})
			if (err == nil) != tc.wantUpload {
				t.Fatalf("Upload = %+v, %v; want success %t", result, err, tc.wantUpload)
			}
		})
	}
}

func TestS3UploadSizeMismatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int64
		want string
	}{
		{"short", 5, "shorter than declared"},
		{"long", 2, "exceeds declared size"},
		{"declared empty", 0, "exceeds declared size"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testS3Repository(t, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected object request: %s %s", r.Method, r.URL)
				w.WriteHeader(http.StatusBadRequest)
			}, false)
			result, err := repo.Upload(t.Context(), &domain.UploadRequest{
				Bucket: "bucket", ObjectName: "object", Reader: strings.NewReader("abc"), Size: tc.size,
			})
			if result != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Upload = %+v, %v; want %q error", result, err, tc.want)
			}
		})
	}
}

func TestS3MultipartUpload(t *testing.T) {
	const size = 17 << 20 // Larger than the transfer manager's multipart threshold.
	for _, tc := range []struct {
		name         string
		declaredSize int64
		rejectPart   bool
		cancel       bool
		wantError    string
	}{
		{name: "success", declaredSize: size},
		{name: "part rejected", declaredSize: -1, rejectPart: true, wantError: "AccessDenied"},
		{name: "short", declaredSize: size + 1, wantError: "shorter than declared"},
		{name: "long", declaredSize: size - 1, wantError: "exceeds declared size"},
		{name: "canceled", declaredSize: size, cancel: true, wantError: "context canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			uploadDone := make(chan struct{})
			var uploaded atomic.Int64
			var aborted atomic.Bool
			repo := testS3Repository(t, func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				w.Header().Set("Content-Type", "application/xml")
				switch {
				case r.Method == http.MethodPost && query.Has("uploads"):
					fmt.Fprint(w, `<InitiateMultipartUploadResult><UploadId>transfer</UploadId></InitiateMultipartUploadResult>`)
				case r.Method == http.MethodPut && query.Get("uploadId") == "transfer" && query.Has("partNumber"):
					n, err := io.Copy(io.Discard, r.Body)
					if err != nil {
						t.Errorf("read part: %v", err)
					}
					uploaded.Add(n)
					if tc.rejectPart && query.Get("partNumber") == "1" {
						w.WriteHeader(http.StatusForbidden)
						fmt.Fprint(w, `<Error><Code>AccessDenied</Code></Error>`)
						return
					}
					w.Header().Set("ETag", `"part"`)
				case r.Method == http.MethodPost && query.Get("uploadId") == "transfer":
					if tc.cancel {
						cancel()
						// Keep the response pending so the client observes cancellation.
						<-uploadDone
						return
					}
					if tc.wantError != "" {
						t.Error("failed upload attempted to complete")
					}
					fmt.Fprint(w, `<CompleteMultipartUploadResult><ETag>&quot;multipart-tag&quot;</ETag></CompleteMultipartUploadResult>`)
				case r.Method == http.MethodDelete && query.Get("uploadId") == "transfer":
					aborted.Store(true)
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected multipart request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			}, false)
			result, err := repo.Upload(ctx, &domain.UploadRequest{
				Bucket: "bucket", ObjectName: "object", Size: tc.declaredSize,
				Reader: io.LimitReader(strings.NewReader(strings.Repeat("x", size)), size),
			})
			close(uploadDone)
			if tc.wantError != "" {
				if result != nil || err == nil || !strings.Contains(err.Error(), tc.wantError) || !aborted.Load() {
					t.Fatalf("expected failed upload and abort; result=%+v err=%v aborted=%t", result, err, aborted.Load())
				}
				if tc.cancel && (!errors.Is(err, context.Canceled) || uploaded.Load() != size) {
					t.Fatalf("expected cancellation after transferring all parts; err=%v uploaded=%d", err, uploaded.Load())
				}
			} else if err != nil || result.Size != size || result.ETag != "multipart-tag" || uploaded.Load() != size || aborted.Load() {
				t.Fatalf("unexpected multipart result %+v, %v; uploaded=%d aborted=%t", result, err, uploaded.Load(), aborted.Load())
			}
		})
	}
}
