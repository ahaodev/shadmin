package bootstrap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"shadmin/internal/conf"

	storepkg "github.com/ahaodev/go-storage"
)

func TestMinioEndpoint(t *testing.T) {
	for _, tc := range []struct {
		address string
		secure  bool
		want    string
	}{
		{"localhost:9000", true, "https://localhost:9000"},
		{"localhost:9000", false, "http://localhost:9000"},
		{"https://localhost:9000/", true, "https://localhost:9000"},
		{"http://localhost:9000", true, ""},
		{"localhost:9000/path", false, ""},
	} {
		got, err := minioEndpoint(tc.address, tc.secure)
		if got != tc.want || (err != nil) != (tc.want == "") {
			t.Errorf("minioEndpoint(%q, %t) = %q, %v; want %q", tc.address, tc.secure, got, err, tc.want)
		}
	}
}

func TestS3Storage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_DEFAULT_PROFILE", "")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "default-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "default-secret")
	t.Setenv("AWS_SESSION_TOKEN", "default-token")
	t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "false")
	t.Setenv("AWS_REQUEST_CHECKSUM_CALCULATION", "when_required")

	for _, tc := range []struct {
		name                      string
		env                       conf.Env
		key, sessionToken, region string
	}{
		{
			name: "AWS default configuration",
			env:  conf.Env{StorageType: conf.StoreTypeS3},
			key:  "default-key", sessionToken: "default-token", region: "us-west-2",
		},
		{
			name: "AWS explicit region and credentials",
			env: conf.Env{
				StorageType: conf.StoreTypeS3, S3Region: "eu-west-1",
				S3AccessKey: "explicit-key", S3SecretKey: "explicit-secret", S3Token: "explicit-token",
			},
			key: "explicit-key", sessionToken: "explicit-token", region: "eu-west-1",
		},
		{
			name: "MinIO path-style requests",
			env: conf.Env{
				StorageType: conf.StoreTypeMinio,
				S3AccessKey: "minio-key", S3SecretKey: "minio-secret", S3Token: "minio-token",
			},
			key: "minio-key", sessionToken: "minio-token", region: "us-east-1",
		},
		{
			name: "MinIO configured region",
			env: conf.Env{
				StorageType: conf.StoreTypeMinio, S3Region: "eu-central-1",
				S3AccessKey: "minio-key", S3SecretKey: "minio-secret",
			},
			key: "minio-key", region: "eu-central-1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := tc.env
			requests := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case requests <- r.Method + " " + r.URL.Path:
				default:
					t.Error("unexpected additional request")
				}
				auth := r.Header.Get("Authorization")
				if !strings.Contains(auth, "Credential="+tc.key+"/") || !strings.Contains(auth, "/"+tc.region+"/s3/") {
					t.Errorf("unexpected signing credential or region: %q", auth)
				}
				if token := r.Header.Get("X-Amz-Security-Token"); token != tc.sessionToken {
					t.Errorf("session token = %q; want %q", token, tc.sessionToken)
				}
				if r.Method == http.MethodPut {
					if body, err := io.ReadAll(r.Body); err != nil || string(body) != "hello" {
						t.Errorf("upload body = %q, %v", body, err)
					}
					w.Header().Set("ETag", `"test-tag"`)
				}
			}))
			defer server.Close()

			wantRequests := []string{"PUT /test-bucket/prefix/file.txt"}
			if env.StorageType == conf.StoreTypeMinio {
				env.S3Address = strings.TrimPrefix(server.URL, "http://")
				wantRequests = append([]string{"HEAD /test-bucket"}, wantRequests...)
			} else {
				t.Setenv("AWS_ENDPOINT_URL_S3", server.URL)
			}
			storage, err := InitStorage(&env)
			if err != nil {
				t.Fatal(err)
			}
			result, err := storage.FileStorage.Upload(context.Background(), &storepkg.UploadRequest{
				Bucket: "test-bucket", Prefix: "prefix", ObjectName: "file.txt",
				Reader: io.LimitReader(strings.NewReader("hello"), 5), Size: -1,
			})
			if err != nil {
				t.Fatal(err)
			}
			want := storepkg.UploadResult{Bucket: "test-bucket", Key: "prefix/file.txt", ObjectName: "prefix/file.txt", ETag: "test-tag", Size: 5}
			if *result != want {
				t.Fatalf("Upload = %+v; want %+v", *result, want)
			}
			var gotRequests []string
			for len(requests) != 0 {
				gotRequests = append(gotRequests, <-requests)
			}
			if !slices.Equal(gotRequests, wantRequests) {
				t.Errorf("requests = %v; want %v", gotRequests, wantRequests)
			}
		})
	}

	t.Setenv("AWS_REGION", "")
	_, err := InitStorage(&conf.Env{StorageType: conf.StoreTypeS3})
	if err == nil || !strings.Contains(err.Error(), "S3_REGION") {
		t.Fatalf("InitStorage without region = %v; want an S3_REGION error", err)
	}
}
