# arxivagent

当前仓库已包含第一批后端骨架代码：

- Go API 入口：`cmd/api`
- 数据库迁移入口：`cmd/migrate`
- PostgreSQL 初始化 SQL：`database/migrations/001_init.sql`
- 本地配置模板：`configs/runtime.local.json.example`

## 当前能力

当前实现完成了这些基础能力：

- 读取本地 JSON 配置
- 连接 PostgreSQL
- 执行 SQL 迁移
- 启动 Gin API 服务
- 提供基础接口：
  - `GET /healthz`
  - `GET /api/papers`
  - `GET /api/papers/:id`
  - `POST /api/papers/:id/parse-generate`
  - `GET /api/drafts`
  - `GET /api/drafts/:id`
  - `PUT /api/drafts/:id`
  - `POST /api/drafts/:id/approve`
  - `POST /api/drafts/:id/reject`
  - `POST /api/drafts/:id/render`
  - `GET /api/site/drafts/today`
  - `GET /api/site/posts/:slug`
  - `GET /api/configs`
  - `GET /api/task-runs`
  - `POST /api/task-runs/daily-discovery/run`
  - `POST /api/task-runs/recommended-papers/parse-generate/run`

## 本地启动

1. 复制配置模板：

```powershell
Copy-Item configs\runtime.local.json.example configs\runtime.local.json
```

2. 启动本地依赖：

```powershell
docker compose up -d
```

3. 安装 PDF 解析依赖：

```powershell
python -m pip install -r worker\requirements.txt
```

4. 如果你要启用 LLM 总结，请在 `configs/runtime.local.json` 中填写：

```json
"llm": {
  "base_url": "https://your-openai-compatible-endpoint",
  "api_key": "your-api-key",
  "model": "your-model-name"
}
```

说明：

- 当前按 OpenAI 兼容接口实现，会请求 `.../v1/chat/completions`
- 如果没有配置 LLM，系统会自动回退到启发式摘要生成

5. 执行迁移：

```powershell
go run ./cmd/migrate
```

6. 启动 API：

```powershell
go run ./cmd/api
```

## 说明

- `configs/runtime.local.json` 已被 `.gitignore` 忽略。
- 当前还没有接入 arXiv 抓取、PDF 解析和文章生成。
- 当前已经接入 arXiv 发现与规则打分的第一版手动触发流程。
- 当前已经接入 PDF 下载、Python 解析脚本调用和 Markdown 草稿生成链路。
- 当前已经接入可配置的 LLM 总结与标题生成，失败时自动回退启发式流程。
- `render` 接口当前使用的是最小 Markdown 渲染逻辑，后续可以替换为正式渲染器。

## 手动触发示例

先跑每日发现：

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/task-runs/daily-discovery/run -ContentType "application/json" -Body "{}"
```

再对当天推荐论文批量解析并生成草稿：

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/task-runs/recommended-papers/parse-generate/run -ContentType "application/json" -Body "{}"
```

也可以单独处理某篇论文：

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/papers/1/parse-generate
```
