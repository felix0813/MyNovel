# MyNovel

MyNovel 是一个「小说记录 + 静态详情页生成」的全栈项目，包含：

- `frontend/`：纯静态前端页面（列表、筛选、编辑）。
- `backend/`：Go + PostgreSQL REST API，负责小说数据管理。
- `serverless/`：阿里云函数，监听 JSON 变更并生成静态 HTML。

> 适合个人阅读管理场景：记录小说名称、平台、链接、状态、评分与简介，并支持生成可公开访问的详情页。

## 项目结构

```text
MyNovel/
├─ frontend/      # 前端页面（index/edit/404）
├─ backend/       # Go 后端 + SQL 初始化脚本
├─ serverless/    # OSS 触发函数（JSON -> HTML）
└─ readme.md
```

## 核心能力

- 小说 CRUD：新增、查询、更新、删除。
- 列表筛选：按关键字与阅读状态过滤。
- 状态管理：未读 / 在读 / 已读。
- 静态详情页：通过 OSS 触发函数生成 HTML 页面。
- 前后端分离：前端直接调用 API，部署灵活。

## 技术栈

- Frontend: HTML / CSS / Vanilla JavaScript
- Backend: Go（`net/http`）+ PostgreSQL + pgx
- Serverless: Node.js（Aliyun OSS SDK）
- Object Storage: Aliyun OSS

## 快速开始（本地）

### 1) 启动后端

请先按 `backend/readme.md` 初始化数据库并启动 API 服务：

```bash
cd backend
go run .
```

默认后端地址：`http://localhost:8080`

### 2) 启动前端（静态文件）

前端是静态资源，建议使用任意静态服务器运行：

```bash
cd frontend
python3 -m http.server 5173
```

然后访问：`http://localhost:5173`

> 注意：当前前端默认 API 地址写在 `frontend/app.js` 与 `frontend/edit.js` 中。若本地联调，请将 `API_BASE` 改成你的后端地址。

## 部署说明（简要）

1. 部署 `backend`，保证 API 可访问。
2. 部署 `frontend` 到静态托管（Nginx / OSS / CDN 等）。
3. 配置 `serverless` 监听 `novel-json` 对象变更。
4. 当后端写入最新 JSON 后，由函数自动渲染并上传小说 HTML 页面。

## 文档索引

- 前端说明：`frontend/readme.md`
- 后端说明：`backend/readme.md`
- 函数说明：`serverless/README.md`

## License

当前仓库未单独声明 License，如需开源请补充 `LICENSE` 文件。
