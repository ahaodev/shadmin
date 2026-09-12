package tokenutil

import (
	"strings"
	"testing"
	"time"

	"shadmin/domain"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef"
	otherSecret = "ffffffffffffffffffffffffffffffff"
)

func newTestUser() *domain.User {
	return &domain.User{
		ID:       "9m4e2mr0ui3e8a215n4g",
		Username: "alice",
		Email:    "alice@example.com",
		IsAdmin:  true,
		Roles:    []string{"admin", "ops"},
	}
}

func TestParseAccessClaims_AllFields(t *testing.T) {
	user := newTestUser()
	token, err := CreateAccessToken(user, testSecret, 60)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}

	claims, err := ParseAccessClaims(token, testSecret)
	if err != nil {
		t.Fatalf("ParseAccessClaims: %v", err)
	}

	if claims.ID != user.ID {
		t.Errorf("ID = %q, want %q", claims.ID, user.ID)
	}
	if claims.JTI() == claims.ID {
		t.Error("JTI() returns the user id; the embedded RegisteredClaims.ID is shadowed")
	}
	if claims.Name != user.Username {
		t.Errorf("Name = %q, want %q", claims.Name, user.Username)
	}
	if claims.Email != user.Email {
		t.Errorf("Email = %q, want %q", claims.Email, user.Email)
	}
	if !claims.IsAdmin {
		t.Error("IsAdmin = false, want true")
	}
	if strings.Join(claims.Roles, ",") != strings.Join(user.Roles, ",") {
		t.Errorf("Roles = %v, want %v", claims.Roles, user.Roles)
	}
	if want := "shadmin:" + user.ID; claims.Subject != want {
		t.Errorf("Subject = %q, want %q", claims.Subject, want)
	}
	if claims.Issuer != "shadmin" {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, "shadmin")
	}
	// jti 是登出黑名单的键，必须存在且不能等同于用户 ID
	if claims.JTI() == "" {
		t.Error("jti is empty, but the logout blacklist depends on it")
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
		t.Errorf("ExpiresAt = %v, want a future time", claims.ExpiresAt)
	}
}

func TestParseRefreshClaims_AllFields(t *testing.T) {
	user := newTestUser()
	token, err := CreateRefreshToken(user, testSecret, 60)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}

	claims, err := ParseRefreshClaims(token, testSecret)
	if err != nil {
		t.Fatalf("ParseRefreshClaims: %v", err)
	}

	if claims.ID != user.ID {
		t.Errorf("ID = %q, want %q", claims.ID, user.ID)
	}
	if claims.JTI() == "" {
		t.Error("jti is empty, but the logout blacklist depends on it")
	}
	if claims.JTI() == claims.ID {
		t.Error("JTI() returns the user id; the embedded RegisteredClaims.ID is shadowed")
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
		t.Errorf("ExpiresAt = %v, want a future time", claims.ExpiresAt)
	}
}

func TestParseClaims_RejectsInvalidTokens(t *testing.T) {
	valid, err := CreateAccessToken(newTestUser(), testSecret, 60)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}
	expired, err := CreateAccessToken(newTestUser(), testSecret, -1)
	if err != nil {
		t.Fatalf("CreateAccessToken(expired): %v", err)
	}

	parts := strings.Split(valid, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format with %d parts", len(parts))
	}
	tampered := parts[0] + "." + parts[1] + "." + strings.Repeat("A", len(parts[2]))

	// alg=none 伪造令牌（算法混淆攻击）
	noneToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, &domain.JwtCustomClaims{
		ID:        "attacker",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build alg=none token: %v", err)
	}

	cases := []struct {
		name   string
		token  string
		secret string
	}{
		{"wrong secret", valid, otherSecret},
		{"expired", expired, testSecret},
		{"tampered signature", tampered, testSecret},
		{"alg=none", noneToken, testSecret},
		{"garbage", "not-a-jwt", testSecret},
		{"empty", "", testSecret},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseAccessClaims(tc.token, tc.secret); err == nil {
				t.Fatal("ParseAccessClaims returned nil error, want rejection")
			}
			if _, err := ParseRefreshClaims(tc.token, tc.secret); err == nil {
				t.Fatal("ParseRefreshClaims returned nil error, want rejection")
			}
		})
	}
}

// 签发端只用 HS256，因此三个解析入口都只接受 HS256。
func TestParseClaims_OnlyAcceptsHS256(t *testing.T) {
	claims := &domain.JwtCustomClaims{
		ID:    "u-1",
		Name:  "alice",
		Email: "alice@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    accessTokenIssuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			ID:        "jti-hmac",
		},
	}

	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign with HS256: %v", err)
	}
	if _, err := ParseAccessClaims(raw, testSecret); err != nil {
		t.Fatalf("ParseAccessClaims rejected HS256: %v", err)
	}
	if _, _, ok := ExtractJTIAndExpiry(raw, testSecret); !ok {
		t.Fatal("ExtractJTIAndExpiry rejected HS256")
	}

	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		t.Run(method.Alg(), func(t *testing.T) {
			raw, err := jwt.NewWithClaims(method, claims).SignedString([]byte(testSecret))
			if err != nil {
				t.Fatalf("sign with %s: %v", method.Alg(), err)
			}
			if _, err := ParseAccessClaims(raw, testSecret); err == nil {
				t.Fatalf("ParseAccessClaims accepted %s, want rejection", method.Alg())
			}
			if _, err := ParseRefreshClaims(raw, testSecret); err == nil {
				t.Fatalf("ParseRefreshClaims accepted %s, want rejection", method.Alg())
			}
			if _, _, ok := ExtractJTIAndExpiry(raw, testSecret); ok {
				t.Fatalf("ExtractJTIAndExpiry accepted %s, want rejection", method.Alg())
			}
		})
	}
}

func TestExtractJTIAndExpiry(t *testing.T) {
	user := newTestUser()
	token, err := CreateRefreshToken(user, testSecret, 60)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}

	jti, exp, ok := ExtractJTIAndExpiry(token, testSecret)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if jti == "" {
		t.Error("jti is empty, but the logout blacklist depends on it")
	}
	if exp.Before(time.Now()) {
		t.Errorf("exp = %v, want a future time", exp)
	}

	if _, _, ok := ExtractJTIAndExpiry(token, otherSecret); ok {
		t.Error("ok = true for a token signed with another secret, want false")
	}
}

// access token 解析必须校验 iss：refresh token 不带 iss，
// 不校验时会被当作 access token 接受，弱化两类令牌的隔离。
func TestParseAccessClaims_RejectsRefreshTokenAndForeignIssuer(t *testing.T) {
	user := newTestUser()
	refreshToken, err := CreateRefreshToken(user, testSecret, 60)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}

	if _, err := ParseAccessClaims(refreshToken, testSecret); err == nil {
		t.Fatal("ParseAccessClaims accepted a refresh token, want rejection")
	}
	// 刷新与登出流程仍依赖 refresh 解析器接受 refresh token
	if _, err := ParseRefreshClaims(refreshToken, testSecret); err != nil {
		t.Fatalf("ParseRefreshClaims rejected refresh token: %v", err)
	}
	if _, _, ok := ExtractJTIAndExpiry(refreshToken, testSecret); !ok {
		t.Fatal("ExtractJTIAndExpiry rejected refresh token")
	}

	for _, issuer := range []string{"", "another-issuer"} {
		name := issuer
		if name == "" {
			name = "missing"
		}
		t.Run("issuer="+name, func(t *testing.T) {
			claims := &domain.JwtCustomClaims{
				ID: user.ID,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    issuer,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					ID:        "jti-issuer",
				},
			}
			raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			if _, err := ParseAccessClaims(raw, testSecret); err == nil {
				t.Fatal("ParseAccessClaims accepted a token with invalid issuer, want rejection")
			}
		})
	}
}
