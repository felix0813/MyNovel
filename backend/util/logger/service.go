package logger

import (
	"context"
	"log"
	"os"
	"strings"
)

type Service struct {
	client  *Client
	appCode string
	env     string
	enabled bool
}

func NewServiceFromEnv(ctx context.Context) *Service {
	c, err := NewFromEnv()
	if err != nil {
		log.Printf("[logger] remote logger disabled: %v", err)
		return &Service{}
	}

	s := &Service{
		client:  c,
		appCode: envOrDefault("LOGGER_APP_CODE", "mynovel-backend"),
		env:     envOrDefault("APP_ENV", "dev"),
		enabled: true,
	}

	registerReq := RegisterAppRequest{
		AppCode:       s.appCode,
		AppName:       envOrDefault("LOGGER_APP_NAME", "MyNovel Backend"),
		Env:           s.env,
		Enabled:       true,
		RetentionDays: 30,
		Description:   envOrDefault("LOGGER_APP_DESC", "MyNovel backend service"),
	}
	if err := s.client.RegisterApp(ctx, registerReq); err != nil {
		log.Printf("[logger] register app failed: %v", err)
	} else {
		log.Printf("[logger] app registered: app_code=%s env=%s", s.appCode, s.env)
	}
	return s
}

func (s *Service) ImportantInfo(ctx context.Context, message string, extra map[string]any) {
	log.Printf("[important] %s", message)
	s.send(ctx, "info", message, extra)
}

func (s *Service) ImportantError(ctx context.Context, message string, err error, extra map[string]any) {
	log.Printf("[important][error] %s err=%v", message, err)
	if extra == nil {
		extra = map[string]any{}
	}
	if err != nil {
		extra["exception"] = err.Error()
	}
	s.send(ctx, "error", message, extra)
}

func (s *Service) SendHTTPLog(ctx context.Context, level string, message string, method, path string, statusCode int, durationMS int64) {
	if !s.enabled || s.client == nil {
		return
	}
	if level == "" {
		level = "info"
	}
	req := SendLogRequest{
		AppCode:    s.appCode,
		Env:        s.env,
		Level:      level,
		Message:    message,
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		DurationMS: durationMS,
	}
	if err := s.client.SendLog(ctx, req); err != nil {
		log.Printf("[logger] send http log failed: %v", err)
	}
}

func (s *Service) send(ctx context.Context, level string, message string, extra map[string]any) {
	if !s.enabled || s.client == nil {
		return
	}
	req := SendLogRequest{
		AppCode: s.appCode,
		Env:     s.env,
		Level:   strings.ToLower(level),
		Message: message,
		Extra:   extra,
	}
	if exception, ok := extra["exception"].(string); ok {
		req.Exception = exception
	}
	if err := s.client.SendLog(ctx, req); err != nil {
		log.Printf("[logger] send log failed: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
