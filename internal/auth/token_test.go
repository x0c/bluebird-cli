package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func envMap(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := LoadConfig(envMap(nil))
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.JWTIssuer != DefaultIssuer {
		t.Errorf("JWTIssuer = %q, want %q", cfg.JWTIssuer, DefaultIssuer)
	}
	if cfg.Configured() {
		t.Error("empty env should not be Configured()")
	}
}

func TestLoadConfig_JWTSecretFallback(t *testing.T) {
	cfg := LoadConfig(envMap(map[string]string{
		"OPENCONDUCTOR_SUPABASE_JWT_SECRET": "shared-secret",
		"BLUEBIRD_USER_ID":                  "user-1",
	}))
	if cfg.JWTSecret != "shared-secret" {
		t.Errorf("expected fallback to OPENCONDUCTOR_SUPABASE_JWT_SECRET, got %q", cfg.JWTSecret)
	}
	if !cfg.Configured() {
		t.Error("expected Configured() true when fallback secret + user id present")
	}
}

func TestLoadConfig_ExplicitSecretWinsOverFallback(t *testing.T) {
	cfg := LoadConfig(envMap(map[string]string{
		"BLUEBIRD_JWT_SECRET":               "explicit-secret",
		"OPENCONDUCTOR_SUPABASE_JWT_SECRET": "shared-secret",
	}))
	if cfg.JWTSecret != "explicit-secret" {
		t.Errorf("expected explicit BLUEBIRD_JWT_SECRET to win, got %q", cfg.JWTSecret)
	}
}

func TestResolve_TokenTakesPriority(t *testing.T) {
	cfg := Config{Token: "tok-abc", JWTSecret: "s", UserID: "u"}
	token, source, err := cfg.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "tok-abc" || source != SourceToken {
		t.Fatalf("got token=%q source=%q, want tok-abc/%q", token, source, SourceToken)
	}
}

func TestResolve_SelfSigned_RoundTrips(t *testing.T) {
	cfg := Config{JWTSecret: "s3cr3t", JWTIssuer: "https://auth.example", UserID: "user-42"}
	tok, source, err := cfg.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != SourceSelfSigned {
		t.Fatalf("source = %q, want %q", source, SourceSelfSigned)
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tok, claims, func(t *jwt.Token) (any, error) {
		return []byte("s3cr3t"), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("https://auth.example"))
	if err != nil || !parsed.Valid {
		t.Fatalf("self-signed token failed to parse: %v", err)
	}
	sub, _ := claims.GetSubject()
	if sub != "user-42" {
		t.Fatalf("sub = %q, want user-42", sub)
	}
	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("no exp claim: %v", err)
	}
	if time.Until(exp.Time) > 6*time.Minute || time.Until(exp.Time) <= 0 {
		t.Fatalf("exp should be ~5min from now, got %v from now", time.Until(exp.Time))
	}
}

func TestResolve_NeitherConfigured_Errors(t *testing.T) {
	cfg := Config{}
	if _, _, err := cfg.Resolve(); err == nil {
		t.Fatal("expected error when neither token nor secret+user configured")
	}
}

func TestVerifySelfSigned_WrongSecretFails(t *testing.T) {
	if err := VerifySelfSigned("right-secret", DefaultIssuer, "u1"); err != nil {
		t.Fatalf("unexpected error signing/verifying with same secret: %v", err)
	}
	// 用不同密钥签发再验证应当失败——模拟 doctor 里"配错 secret"的场景。
	tok, err := selfSign("secret-a", DefaultIssuer, "u1")
	if err != nil {
		t.Fatalf("selfSign failed: %v", err)
	}
	_, err = jwt.Parse(tok, func(t *jwt.Token) (any, error) { return []byte("secret-b"), nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err == nil {
		t.Fatal("expected verification failure with mismatched secret")
	}
}
