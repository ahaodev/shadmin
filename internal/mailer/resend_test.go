package mailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/resend/resend-go/v4"
)

func TestResendInvitationMailerUsesConfiguredSenderAndEscapesMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Errorf("request = %s %s, want POST /emails", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want configured API key", got)
		}
		var body struct {
			From    string   `json:"from"`
			To      []string `json:"to"`
			Subject string   `json:"subject"`
			Html    string   `json:"html"`
			Text    string   `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if body.From != "Shadmin <invite@example.com>" || len(body.To) != 1 || body.To[0] != "user@example.com" {
			t.Errorf("message sender/recipient = %q/%v", body.From, body.To)
		}
		if strings.Contains(body.Html, "<script>") || !strings.Contains(body.Html, "&lt;script&gt;") {
			t.Errorf("invitation message was not HTML-escaped: %s", body.Html)
		}
		if !strings.Contains(body.Text, "https://example.com/accept#token=opaque") {
			t.Errorf("text body does not contain the invitation URL: %s", body.Text)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email-1"}`))
	}))
	defer server.Close()

	client := resend.NewCustomClient(server.Client(), "test-key")
	client.BaseURL, _ = url.Parse(server.URL)
	mailer := &ResendInvitationMailer{client: client, from: "Shadmin <invite@example.com>"}

	if err := mailer.SendInvitation(context.Background(), "user@example.com", "https://example.com/accept#token=opaque", "<script>alert(1)</script>"); err != nil {
		t.Fatalf("SendInvitation: %v", err)
	}
}
