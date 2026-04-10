# 文档拆分说明

原始总文档保留为：

- `docs/arxiv-wechat-architecture.md`

为了便于后续逐模块实现，已将方案拆分为一组更适合“按规格生成代码”的文档。

建议阅读顺序：

1. `docs/00-overview.md`
2. `docs/01-business-flow.md`
3. `docs/02-system-architecture.md`
4. `docs/03-database-schema.md`
5. `docs/04-api-design.md`
6. `docs/05-admin-console.md`
7. `docs/06-paper-discovery-and-ranking.md`
8. `docs/07-pdf-parsing-and-generation.md`
9. `docs/08-wechat-publishing.md`
10. `docs/09-deployment-and-ops.md`
11. `docs/10-dev-plan.md`

## 推荐协作方式

后续按下面节奏推进最稳：

1. 你选择一个文档补充业务细节
2. 我先把该文档收敛成可实现规格
3. 我基于该文档生成对应代码
4. 缺少的信息只围绕当前模块向你要

## 推荐实现顺序

第一阶段先做后端基础：

- `03-database-schema.md`
- `04-api-design.md`
- `01-business-flow.md`
- `02-system-architecture.md`

第二阶段做核心能力：

- `06-paper-discovery-and-ranking.md`
- `07-pdf-parsing-and-generation.md`
- `08-wechat-publishing.md`

第三阶段做运营台与运维：

- `05-admin-console.md`
- `09-deployment-and-ops.md`
- `10-dev-plan.md`

## 你给我内容时的建议格式

每次只需要给某一个文件补这几类信息：

- 目标：这个模块必须解决什么问题
- 边界：这个模块明确不做什么
- 输入：依赖哪些表、接口、配置、外部系统
- 输出：产出哪些数据、状态、页面或接口
- 规则：校验、状态流转、异常处理、权限要求
- 样例：请求响应、JSON、页面字段、Prompt、报错示例

## 当前建议的第一步

建议你优先补：

1. `docs/03-database-schema.md`
2. `docs/04-api-design.md`

这两份定下来之后，我就可以开始搭项目骨架并生成第一批代码。

