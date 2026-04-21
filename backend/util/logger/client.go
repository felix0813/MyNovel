package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultTimeout = 5 * time.Second

// Client 是 PG-Logger 的轻量 HTTP 客户端。
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewFromEnv 从环境变量 LOGGER_SERVER_URL 读取日志服务地址。
func NewFromEnv() (*Client, error) {
	baseURL := os.Getenv("LOGGER_SERVER_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("LOGGER_SERVER_URL is required")
	}
	return New(baseURL), nil
}

// New 使用传入的 baseURL 创建客户端。
func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// RegisterAppRequest 对应 POST /logger/apps。
type RegisterAppRequest struct {
	AppCode       string `json:"app_code"`
	AppName       string `json:"app_name"`
	Env           string `json:"env"`
	Enabled       bool   `json:"enabled"`
	RetentionDays int    `json:"retention_days"`
	Description   string `json:"description"`
}

// SendLogRequest 对应 POST /logger/logs。
type SendLogRequest struct {
	AppCode    string         `json:"app_code"`
	Env        string         `json:"env"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	Path       string         `json:"path,omitempty"`
	Method     string         `json:"method,omitempty"`
	StatusCode int            `json:"status_code,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	Exception  string         `json:"exception,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

// RegisterApp 注册应用。
func (c *Client) RegisterApp(ctx context.Context, input RegisterAppRequest) error {
	if input.AppCode == "" || input.AppName == "" || input.Env == "" {
		return fmt.Errorf("app_code, app_name and env are required")
	}
	return c.postJSON(ctx, "/logger/apps", input)
}

// SendLog 发送日志 JSON。
func (c *Client) SendLog(ctx context.Context, input SendLogRequest) error {
	if input.AppCode == "" || input.Level == "" || input.Message == "" {
		return fmt.Errorf("app_code, level and message are required")
	}
	return c.postJSON(ctx, "/logger/logs", input)
}

func (c *Client) postJSON(ctx context.Context, path string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: status=%d", resp.StatusCode)
	}

	return nil
}
