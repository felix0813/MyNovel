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

> 建议在函数计算控制台中按下表配置；未配置且无默认值时，函数会在运行时抛错。

| 变量名 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `OSS_ENDPOINT` | 是 | - | OSS 访问域名，例如 `oss-cn-hangzhou.aliyuncs.com`。 |
| `OSS_ACCESS_KEY_ID` | 是 | - | OSS AccessKey ID。 |
| `OSS_ACCESS_KEY_SECRET` | 是 | - | OSS AccessKey Secret。 |
| `TARGET_HTML_BUCKET` | 否 | `novels-html` | 生成的 HTML 要写入的目标 Bucket。 |
| `TARGET_HTML_PREFIX` | 否 | `novels` | 目标 Bucket 下的目录前缀（会自动去掉首尾 `/`）。 |
| `LOGGER_SERVER_URL` | 否 | - | 远端日志服务地址（配置后会调用 `${LOGGER_SERVER_URL}/logger/apps` 和 `${LOGGER_SERVER_URL}/logger/logs`）。 |
| `LOGGER_APP_CODE` | 否 | `mynovel-serverless` | 远端日志应用编码。 |
| `LOGGER_APP_NAME` | 否 | `MyNovel Serverless` | 远端日志应用名称（用于自动注册）。 |
| `LOGGER_APP_DESC` | 否 | `MyNovel OSS trigger` | 远端日志应用描述。 |
| `LOGGER_RETENTION_DAYS` | 否 | `30` | 远端日志保留天数（注册应用时使用）。 |
| `APP_ENV` | 否 | `dev` | 当前环境标识（会写入远端日志 `env` 字段）。 |

### 推荐配置示例

```env
OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
OSS_ACCESS_KEY_ID=your-ak
OSS_ACCESS_KEY_SECRET=your-sk
TARGET_HTML_BUCKET=novels-html
TARGET_HTML_PREFIX=novels

# 可选：远端日志
LOGGER_SERVER_URL=https://your-log-server.example.com
LOGGER_APP_CODE=mynovel-serverless
LOGGER_APP_NAME=MyNovel Serverless
LOGGER_APP_DESC=MyNovel OSS trigger
LOGGER_RETENTION_DAYS=30
APP_ENV=prod
```

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
