# MyNovel Frontend

这是 MyNovel 的前端模块，使用原生 HTML/CSS/JavaScript 构建，提供小说列表管理与编辑页面。

## 页面说明

- `index.html`：小说列表页（搜索、状态筛选、刷新、删除）。
- `edit.html`：新增/编辑页（表单提交、删除）。
- `404.html`：静态 404 页面。

## 主要功能

- 展示小说列表：名称、平台、状态、评分、链接。
- 支持搜索与状态筛选。
- 支持新增、编辑、删除小说。
- 名称支持跳转静态详情页：`/novels/{id}.html`。

## 目录结构

```text
frontend/
├─ index.html
├─ edit.html
├─ 404.html
├─ app.js        # 列表页逻辑
├─ edit.js       # 编辑页逻辑
├─ styles.css    # 全局样式
├─ favicon.svg
└─ readme.md
```

## API 对接

前端通过 `fetch` 访问后端接口，默认 API 基地址为：

```js
const API_BASE = "https://wzfly.top/myNovel/api";
```

位置：

- `app.js`
- `edit.js`

如果你在本地联调，请将其改为本地地址，例如：

```js
const API_BASE = "http://localhost:8080/api";
```

## 本地运行

在 `frontend` 目录启动静态文件服务：

```bash
cd frontend
python3 -m http.server 5173
```

浏览器访问：`http://localhost:5173`

## 与后端约定

### 状态值

- `unread`：未读
- `reading`：在读
- `finished`：已读

### 评分范围

- `0 ~ 10`（整数或数字）

### 关键接口

- `GET /api/novels?q=&status=`：列表
- `GET /api/novels/{id}`：详情
- `POST /api/novels`：新增
- `PUT /api/novels/{id}`：更新
- `DELETE /api/novels/{id}`：删除

## 注意事项

- 删除操作有确认弹窗，且不可恢复。
- 列表页中名称和“详情页”链接依赖后端数据中的 `id`。
- 若出现跨域问题，请在后端配置 `CORS_ALLOW_ORIGIN`。
