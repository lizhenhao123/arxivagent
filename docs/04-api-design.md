# 04 API Design

## 目标

定义前后端接口、任务操作接口、配置接口和站内审阅接口，作为后端控制器与前端数据接入依据。

## 接口分组建议

- 论文池接口
- 草稿接口
- 站内页面接口
- 配置接口
- 日志与任务接口

## 最小接口集合

### 论文池

- `GET /api/papers`
- `GET /api/papers/:id`
- `POST /api/papers/:id/rescore`
- `POST /api/papers/:id/recommend`

### 草稿

- `GET /api/drafts`
- `GET /api/drafts/:id`
- `PUT /api/drafts/:id`
- `POST /api/drafts/:id/approve`
- `POST /api/drafts/:id/reject`
- `POST /api/drafts/:id/render`

### 站内页面

- `GET /api/site/drafts/today`
- `GET /api/site/posts/:slug`

### 配置

- `GET /api/configs`
- `PUT /api/configs/:key`
- `GET /api/prompt-templates`
- `PUT /api/prompt-templates/:id`

### 任务与日志

- `GET /api/task-runs`
- `GET /api/task-runs/:id`
- `POST /api/task-runs/daily-discovery/run`

## 接口设计要求

- 返回统一响应结构
- 错误码可枚举
- 状态变更接口必须校验前置状态
- 支持分页、筛选和排序
- V1 不做登录态，默认内部可访问

## 推荐统一响应格式

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

## 推荐分页格式

```json
{
  "items": [],
  "page": 1,
  "page_size": 20,
  "total": 100
}
```

## 与代码生成的关系

本文件会直接影响：

- Go router
- handler / service / dto
- 前端 API client
- Swagger 或 OpenAPI 文档

## 补充内容

- 目前不需要登陆，只需要提供一个页面，这个页面可以每天都给我一份你整理好的文章，把内容发给我就行，内容保存格式你可以自己考虑，自己决定。但是要和03-database的设计的格式保持一致。

## 当前结论

- V1 不需要 JWT 登录
- V1 不需要 RBAC
- 每天需要有一个站内页面可查看当日整理好的文章
- 草稿主存储格式与 `03-database-schema.md` 保持一致，采用 Markdown 优先
- 站内展示时可将 Markdown 渲染为 HTML

## 下一步建议补充

为了把接口文档转成可写代码的 OpenAPI，我下一步还需要你补：

- 论文列表页需要哪些筛选项
- 草稿详情页是否需要展示所有图像
- 站内页面 URL 风格是按日期还是按 slug
- 是否需要“重新生成总结”和“重新解析 PDF”按钮
