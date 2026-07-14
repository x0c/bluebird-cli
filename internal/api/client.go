// Package api 是 Bluebird 后端的 HTTP 客户端：只负责发请求、解析 JSON、
// 把 HTTP 状态映射成 output.CmdError。不包含业务语义（地名解析、幂等收敛等），
// 那些留给 internal/place 与 internal/cmdtree 的各资源命令。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/x0c/bluebird-cli/internal/auth"
)

// Client 持有一次命令执行所需的 HTTP 连接信息；不缓存 token 之外的任何状态。
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	tokenSrc   auth.Source
	userAgent  string
}

// NewClient 用鉴权配置构造一个已解析出 Bearer token 的客户端。
func NewClient(cfg auth.Config, appVersion string) (*Client, error) {
	token, source, err := cfg.Resolve()
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		token:      token,
		tokenSrc:   source,
		userAgent:  "bluebird-cli/" + appVersion,
	}, nil
}

// NewUnauthenticatedClient 用于 /healthz 等无需鉴权的探测。
func NewUnauthenticatedClient(baseURL string, timeout time.Duration, appVersion string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		userAgent:  "bluebird-cli/" + appVersion,
	}
}

// TokenSource 报告本次请求实际使用的令牌来源，供 doctor/--dry-run 展示（不含密钥值）。
func (c *Client) TokenSource() auth.Source {
	return c.tokenSrc
}

// Get 发起 GET 请求；query 为 nil 时不带查询参数；out 非 nil 时解码响应体到 out。
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

// GetRaw 发起 GET 请求并返回原始响应体，供需要区分「204 无数据」与「有数据」的端点使用
// （如 /api/air-quality/current、/api/air-quality/stations 后端用 204 表示暂无可用站点）。
func (c *Client) GetRaw(ctx context.Context, path string, query url.Values) ([]byte, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, path, query, nil, &raw)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// Post 发起 POST 请求，body 会被序列化为 JSON。
func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

// Put 发起 PUT 请求，body 会被序列化为 JSON。
func (c *Client) Put(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, http.MethodPut, path, nil, body, out)
}

// Delete 发起 DELETE 请求。
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	full := c.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return NewMarshalError(err)
		}
		reqBody = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, reqBody)
	if err != nil {
		return NewRequestError(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MapTransportError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return MapTransportError(err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out != nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, out); err != nil {
				return NewDecodeError(err)
			}
		}
		return nil
	}

	return MapHTTPError(resp.StatusCode, raw)
}
