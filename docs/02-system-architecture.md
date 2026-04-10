# 02 System Architecture

## 目标

定义系统服务边界、模块职责、调用关系和部署起点，避免后续实现出现职责重叠。

## 服务划分

建议采用“Go 主服务 + Python Worker”的结构：

- `admin-api`
- `scheduler-service`
- `crawler-service`
- `paper-scoring-service`
- `article-generator-service`
- `wechat-publisher-service`
- `pdf-parser-service`（Python）

## 模块职责

### `admin-api`

- 提供前端接口
- 管理审核流
- 保存人工编辑内容
- 暴露日志和配置接口

### `scheduler-service`

- 每日定时任务
- 补偿任务
- 重试任务

### `crawler-service`

- 调用 arXiv API
- 候选论文入库
- 版本快照记录

### `paper-scoring-service`

- 规则过滤
- 打分
- LLM 重排

### `article-generator-service`

- 调用解析结果
- 生成审稿 JSON
- 生成公众号 HTML

### `wechat-publisher-service`

- 微信素材上传
- 草稿创建
- 发布和状态查询

### `pdf-parser-service`

- 下载 PDF
- 解析章节、图表标题和结构化文本

## 部署建议

V1 先用 Docker Compose，服务可先部署在单机环境：

- PostgreSQL
- Redis
- Go API / Scheduler
- Python Worker
- Frontend

## 设计原则

- Agent 只是职责抽象，不作为自由自治单元实现
- 每个模块只做单一职责
- 所有结果都应落库
- 可先单仓库组织，后续再按需要拆服务

## 与代码生成的关系

本文件会直接影响：

- 项目目录结构
- 服务间接口定义
- Docker Compose 编排
- 后端包结构

## 需要你补充的信息

- 是否接受 V1 先做单体仓库
- Go 和 Python 是否分仓还是同仓
- 前端是否独立部署
- LLM 调用由 Go 直接发起还是统一走 Python Worker

