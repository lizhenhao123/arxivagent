# 10 Dev Plan

## 目标

定义推荐的实现顺序、阶段交付物、模块依赖和验收标准，避免实现顺序混乱。

## 推荐阶段

### 阶段一：基础骨架

- 初始化仓库结构
- 建立数据库迁移
- 建立后端 API 基础框架
- 建立前端基础框架

### 阶段二：核心业务闭环

- 论文发现
- 规则过滤与评分
- PDF 解析
- 草稿生成

### 阶段三：审核与发布

- 编辑器
- 审核流
- 发布链路
- 重试和日志

### 阶段四：工程化

- Docker Compose
- 监控告警
- 单元测试
- 集成测试

## 建议先后顺序

1. `03-database-schema.md`
2. `04-api-design.md`
3. `01-business-flow.md`
4. `02-system-architecture.md`
5. `06-paper-discovery-and-ranking.md`
6. `07-pdf-parsing-and-generation.md`
7. `08-wechat-publishing.md`
8. `05-admin-console.md`
9. `09-deployment-and-ops.md`

## 第一批代码建议

建议我先生成：

- 项目目录结构
- PostgreSQL 建表 SQL
- Go 后端基础框架
- 基础 API 路由
- Docker Compose 初稿

## 验收方式

每一阶段都建议有可验证结果：

- 能启动服务
- 能写入数据库
- 能跑一条任务
- 能生成一条草稿
- 能完成一次发布模拟

## 需要你补充的信息

- 你想先做后端还是先搭全栈骨架
- 是否希望我直接开始建仓库结构
- 是否要优先把数据库和 API 固化下来
- 是否需要我先产出 SQL 和 OpenAPI 草稿

