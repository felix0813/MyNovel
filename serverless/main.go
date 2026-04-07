package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type ossEvent struct {
	Events []struct {
		EventName string `json:"eventName"`
		Oss       struct {
			Bucket struct {
				Name string `json:"name"`
			} `json:"bucket"`
			Object struct {
				Key string `json:"key"`
			} `json:"object"`
		} `json:"oss"`
	} `json:"events"`
}

type syncPayload struct {
	GeneratedAt string  `json:"generated_at"`
	Novels      []Novel `json:"novels"`
}

type Novel struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	URL      string `json:"url"`
	File     string `json:"file"`
	Status   string `json:"status"`
	Rating   int    `json:"rating"`
}

var pageTpl = template.Must(template.New("novels").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>小说展示</title>
  <style>
    body{font-family:Arial,"Microsoft YaHei";background:#f8fafc;margin:0;color:#1e293b}
    header{padding:18px 24px;background:#4338ca;color:#fff}
    main{max-width:1000px;margin:20px auto;padding:0 12px}
    .card{background:#fff;border-radius:10px;padding:12px 16px;margin-bottom:12px;box-shadow:0 4px 16px rgba(0,0,0,.06)}
    .meta{color:#64748b;font-size:13px}
    .row{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap}
    a{color:#4f46e5;text-decoration:none}
  </style>
</head>
<body>
  <header>
    <h1>📚 小说展示页</h1>
    <div><a style="color:#c7d2fe" href="/index.html">返回管理首页</a></div>
  </header>
  <main>
    {{range .Novels}}
      <div class="card">
        <div class="row"><h3>{{.Name}}</h3><strong>{{status .Status}}</strong></div>
        <div class="meta">平台：{{.Platform}} | 评分：{{.Rating}}/10</div>
        <div class="meta">文件：{{.File}}</div>
        {{if .URL}}<div><a href="{{.URL}}" target="_blank">阅读链接</a></div>{{end}}
      </div>
    {{else}}
      <div class="card">暂无小说数据</div>
    {{end}}
  </main>
</body>
</html>`))

func main() {
	// Aliyun FC custom runtime: use HTTP mode to receive OSS trigger event payload.
	httpStart(handle)
}

func handle(ctx context.Context, payload []byte) (string, error) {
	endpoint := os.Getenv("OSS_ENDPOINT")
	ak := os.Getenv("OSS_ACCESS_KEY_ID")
	sk := os.Getenv("OSS_ACCESS_KEY_SECRET")
	targetBucketName := env("TARGET_HTML_BUCKET", "novels-html")
	targetPrefix := strings.Trim(env("TARGET_HTML_PREFIX", "novels"), "/")

	client, err := oss.New(endpoint, ak, sk)
	if err != nil {
		return "", err
	}

	ev, err := parseEvent(payload)
	if err != nil {
		return "", err
	}
	if len(ev.Events) == 0 {
		return "", fmt.Errorf("event is empty")
	}
	sourceBucket := ev.Events[0].Oss.Bucket.Name
	sourceObject := ev.Events[0].Oss.Object.Key

	srcBucket, err := client.Bucket(sourceBucket)
	if err != nil {
		return "", err
	}
	content, err := srcBucket.GetObject(sourceObject)
	if err != nil {
		return "", err
	}
	defer content.Close()

	var p syncPayload
	if err := json.NewDecoder(content).Decode(&p); err != nil {
		return "", err
	}
	sort.Slice(p.Novels, func(i, j int) bool { return p.Novels[i].Rating > p.Novels[j].Rating })

	page, err := renderHTML(p)
	if err != nil {
		return "", err
	}

	targetBucket, err := client.Bucket(targetBucketName)
	if err != nil {
		return "", err
	}
	key := targetPrefix + "/index.html"
	if err := targetBucket.PutObject(key, strings.NewReader(page), oss.ContentType("text/html; charset=utf-8")); err != nil {
		return "", err
	}
	return fmt.Sprintf("generated %s", key), nil
}

func parseEvent(b []byte) (ossEvent, error) {
	var ev ossEvent
	if err := json.Unmarshal(b, &ev); err != nil {
		return ev, err
	}
	return ev, nil
}

func renderHTML(data syncPayload) (string, error) {
	funcs := template.FuncMap{
		"status": func(s string) string {
			switch s {
			case "unread":
				return "未读"
			case "reading":
				return "在读"
			default:
				return "已读"
			}
		},
	}
	tpl, err := pageTpl.Clone()
	if err != nil {
		return "", err
	}
	tpl = tpl.Funcs(funcs)
	var sb strings.Builder
	if err := tpl.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// httpStart wraps a minimal HTTP entry to keep example self-contained.
// In FC, you can replace with official runtime SDK startup.
func httpStart(fn func(context.Context, []byte) (string, error)) {
	// placeholder to keep compile errors away in simplified sample.
	// For actual deployment, integrate with aliyun fc runtime SDK.
	log.Println("serverless handler ready; integrate with FC runtime bootstrap")
	_ = fn
}
