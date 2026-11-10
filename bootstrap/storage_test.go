package bootstrap

import (
	"shadmin/internal/conf"
	"testing"
)

func TestMinioTLSConfiguration(t *testing.T) {
	for _, secure := range []bool{true, false} {
		storage, err := InitStorage(&conf.Env{StorageType: conf.StoreTypeMinio, S3Address: "localhost:9000", S3Secure: secure})
		if err != nil {
			t.Fatal(err)
		}
		scheme := "http"
		if secure {
			scheme = "https"
		}
		if got := storage.MinioClient.EndpointURL().Scheme; got != scheme {
			t.Fatalf("scheme=%s, want %s", got, scheme)
		}
	}
}
