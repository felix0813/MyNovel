# Aliyun OSS Trigger Function（Node.js）

该函数用于监听 `novel-json` 桶中的 JSON 对象变更事件：

1. 读取事件中的 JSON（默认由 backend 写入 `novels/latest.json`）。
2. 将小说数据渲染为静态 HTML。
3. 上传到 `novels-html` 桶下的 `novels/index.html`。

## Runtime 与入口

- Runtime: Node.js
- Entry: `index.mjs`
- Handler: `handler`

## 环境变量

- `OSS_ENDPOINT`
- `OSS_ACCESS_KEY_ID`
- `OSS_ACCESS_KEY_SECRET`
- `TARGET_HTML_BUCKET`（默认 `novels-html`）
- `TARGET_HTML_PREFIX`（默认 `novels`）

## 事件格式

函数按 OSS 触发器的标准事件解析：

```json
{
  "events": [
    {
      "eventName": "ObjectCreated:PutObject",
      "oss": {
        "bucket": { "name": "novel-json" },
        "object": { "key": "novels/latest.json" }
      }
    }
  ]
}
```
