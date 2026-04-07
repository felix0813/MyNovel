# MyNovel Backend

这是一个基于 Go + PostgreSQL 的小说管理后端服务，提供 REST API 用于小说记录的增删改查，并支持在数据变更后将最新列表同步为 JSON 到阿里云 OSS（可选）。

## 功能概览

- 健康检查接口：`GET /healthz`。
- 小说列表查询：`GET /api/novels`，支持按关键字与状态筛选。
- 小说详情查询：`GET /api/novels/{id}`。
- 新增小说：`POST /api/novels`。
- 更新小说：`PUT /api/novels/{id}`。
- 删除小说：`DELETE /api/novels/{id}`。
- CORS 支持：允许配置跨域来源。
- 可选 OSS 同步：在创建/更新/删除后，将最新小说列表上传为 JSON 文件。

## 技术栈

- Go 1.24+
- PostgreSQL
- pgx / pgxpool
- 阿里云 OSS SDK（可选）

## 项目文件说明

- `main.go`：HTTP 服务入口、路由、业务处理与 OSS 同步逻辑。
- `init.sql`：数据库初始化脚本（类型、表、索引、视图、触发器）。
- `go.mod`：依赖管理。

## 快速开始

### 1) 准备数据库

创建数据库后执行初始化脚本：

```bash
psql "postgres://postgres:postgres@localhost:5432/mynovel?sslmode=disable" -f backend/init.sql
```

### 2) 配置环境变量

最少只需要 `DATABASE_URL`（不配时也有默认值），其余按需配置。

### 3) 启动服务

```bash
cd backend
go run .
```

默认监听地址：`http://localhost:8080`

## 环境变量说明

| 变量名 | 是否必需 | 默认值 | 说明 |
|---|---|---|---|
| `DATABASE_URL` | 否（有默认） | `postgres://postgres:postgres@localhost:5432/mynovel?sslmode=disable` | PostgreSQL 连接串。 |
| `ADDR` | 否 | `:8080` | HTTP 服务监听地址。 |
| `CORS_ALLOW_ORIGIN` | 否 | `*` | CORS 允许来源，例如 `http://localhost:3000`。 |
| `OSS_ENDPOINT` | OSS 同步时必需 | 无 | 阿里云 OSS Endpoint，例如 `oss-cn-hangzhou.aliyuncs.com`。 |
| `OSS_ACCESS_KEY_ID` | OSS 同步时必需 | 无 | 阿里云 AccessKey ID。 |
| `OSS_ACCESS_KEY_SECRET` | OSS 同步时必需 | 无 | 阿里云 AccessKey Secret。 |
| `OSS_JSON_BUCKET` | 否 | `novel-json` | OSS Bucket 名称。 |
| `OSS_JSON_OBJECT` | 否 | `novels/latest.json` | OSS 中 JSON 对象路径。 |

> 说明：如果 `OSS_ENDPOINT` / `OSS_ACCESS_KEY_ID` / `OSS_ACCESS_KEY_SECRET` 任一未配置，后端会自动跳过 OSS 上传，不影响 API 正常使用。

## API 简要说明

### 健康检查

- `GET /healthz`
- 返回示例：

```json
{"status":"ok"}
```

### 查询小说列表

- `GET /api/novels?q=关键词&status=unread|reading|finished`
- 参数均可选：
  - `q`：按 `name` 或 `platform` 模糊匹配。
  - `status`：状态精确匹配。

### 新增小说

- `POST /api/novels`
- 请求体示例：

```json
{
  "name": "凡人修仙传",
  "platform": "起点",
  "url": "https://example.com/book/1",
  "file": "/books/fanren.epub",
  "description": "修仙题材",
  "status": "reading",
  "rating": 9
}
```

字段约束：
- `name` 必填。
- `status` 必须是 `unread` / `reading` / `finished`。
- `rating` 取值范围 `0-10`。

### 更新 / 删除 / 详情

- `GET /api/novels/{id}`
- `PUT /api/novels/{id}`（请求体同新增）
- `DELETE /api/novels/{id}`

## 数据库说明

`init.sql` 会创建：

- 枚举类型：`novel_status`（`unread` / `reading` / `finished`）。
- 主表：`novels`。
- 索引：
  - `idx_novels_name`（全文检索向量索引）
  - `idx_novels_status`（状态索引）
- 视图：`v_novel_stats`（按状态统计数量、平均评分、最后更新时间）。
- 触发器：更新数据时自动刷新 `updated_at`。

## 常见问题

1. **启动报数据库连接错误**
   - 检查 `DATABASE_URL`、数据库是否启动、账号权限是否正确。

2. **新增/更新成功但提示 sync failed**
   - 这通常是 OSS 配置错误导致上传失败；如不需要 OSS，可不配置 OSS 三个必需变量。

3. **前端跨域失败**
   - 设置 `CORS_ALLOW_ORIGIN` 为前端地址（如 `http://localhost:3000`），避免使用生产环境下的 `*`。
