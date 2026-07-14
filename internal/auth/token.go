// Package auth 解析 bluebird CLI 的鉴权配置并按优先级取出可用的 Bearer token。
// 密钥只从环境变量读取，禁止落盘、禁止进日志。
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 默认值：生产地址、Supabase issuer、自签/请求超时。
const (
	DefaultBaseURL        = "https://aura.caozc.top"
	DefaultIssuer         = "https://auth.caozc.top"
	DefaultTimeoutSeconds = 30
	selfSignTTL           = 5 * time.Minute
)

// Source 描述令牌来源，供 doctor/--dry-run 展示，不含密钥值本身。
type Source string

const (
	SourceNone       Source = "none"
	SourceToken      Source = "BLUEBIRD_TOKEN"
	SourceSelfSigned Source = "self_signed"
)

// Config 是从环境变量加载的鉴权与连接配置。
type Config struct {
	BaseURL   string
	Token     string
	JWTSecret string
	JWTIssuer string
	UserID    string
	Timeout   time.Duration
}

// LoadConfig 从环境变量读取配置；getenv 传入以便测试注入，不直接依赖 os.Getenv。
func LoadConfig(getenv func(string) string) Config {
	return Config{
		BaseURL:   firstNonEmpty(getenv("BLUEBIRD_BASE_URL"), DefaultBaseURL),
		Token:     getenv("BLUEBIRD_TOKEN"),
		JWTSecret: firstNonEmpty(getenv("BLUEBIRD_JWT_SECRET"), getenv("OPENCONDUCTOR_SUPABASE_JWT_SECRET")),
		JWTIssuer: firstNonEmpty(getenv("BLUEBIRD_JWT_ISSUER"), DefaultIssuer),
		UserID:    getenv("BLUEBIRD_USER_ID"),
		Timeout:   DefaultTimeoutSeconds * time.Second,
	}
}

// Configured 报告是否存在任一可用的鉴权路径，不做实际签名。
func (c Config) Configured() bool {
	if c.Token != "" {
		return true
	}
	return c.JWTSecret != "" && c.UserID != ""
}

// Resolve 按优先级解析出可用于 Authorization 头的 Bearer token：
// 1. BLUEBIRD_TOKEN 原样使用；2. JWTSecret+UserID 自签短期 HS256；3. 都没有则报错。
func (c Config) Resolve() (token string, source Source, err error) {
	if c.Token != "" {
		return c.Token, SourceToken, nil
	}
	if c.JWTSecret != "" && c.UserID != "" {
		tok, err := selfSign(c.JWTSecret, c.JWTIssuer, c.UserID)
		if err != nil {
			return "", SourceNone, err
		}
		return tok, SourceSelfSigned, nil
	}
	return "", SourceNone, errors.New("鉴权未配置")
}

func selfSign(secret, issuer, userID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID,
		"iss": issuer,
		"iat": now.Unix(),
		"exp": now.Add(selfSignTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifySelfSigned 是 doctor 离线自验：用同一份密钥签发并立即验证签名链路，不触网。
func VerifySelfSigned(secret, issuer, userID string) error {
	tok, err := selfSign(secret, issuer, userID)
	if err != nil {
		return err
	}
	_, err = jwt.Parse(tok, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(issuer))
	return err
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
