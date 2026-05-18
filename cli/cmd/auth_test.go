package cmd

import "testing"

func TestResolveVerificationURI(t *testing.T) {
	tests := []struct {
		name            string
		serverURL       string
		verificationURI string
		want            string
	}{
		{
			name:            "empty falls back to auth device path",
			serverURL:       "http://localhost:55667",
			verificationURI: "",
			want:            "http://localhost:55667/auth/device",
		},
		{
			name:            "relative path resolved against server",
			serverURL:       "http://localhost:55667/",
			verificationURI: "/auth/device",
			want:            "http://localhost:55667/auth/device",
		},
		{
			name:            "absolute uri unchanged",
			serverURL:       "http://localhost:55667",
			verificationURI: "https://shadmin.example.com/auth/device",
			want:            "https://shadmin.example.com/auth/device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVerificationURI(tt.serverURL, tt.verificationURI); got != tt.want {
				t.Fatalf("resolveVerificationURI() = %q, want %q", got, tt.want)
			}
		})
	}
}
