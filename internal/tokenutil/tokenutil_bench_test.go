package tokenutil

import "testing"

func BenchmarkParseAccessClaims(b *testing.B) {
	token, err := CreateAccessToken(newTestUser(), testSecret, 60)
	if err != nil {
		b.Fatalf("CreateAccessToken: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseAccessClaims(token, testSecret); err != nil {
			b.Fatal(err)
		}
	}
}
