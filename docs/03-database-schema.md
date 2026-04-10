# 03 Database Schema

## 目标

定义 PostgreSQL 核心表、字段类型、索引、唯一约束、状态字段和 JSONB 结构，作为后续建表 SQL、迁移脚本、Go `model/repository` 实现依据。

这份文档会直接服务于 V1 的第一阶段实现：

1. 每天从 arXiv 抓取候选论文
2. 选出当天最优 1 篇
3. 下载 PDF
4. 解析 PDF，保留论文全文元信息
5. 保存论文中的全部可提取图像
6. 生成结构化总结
7. 生成站内审核稿，正文主格式为 Markdown

---

## 设计结论

结合当前需求，数据库设计采用以下原则：

- 以 `papers` 为主实体
- 所有关键中间结果都落库
- 大段结构化内容优先使用 `JSONB`
- 第一篇可发文章所需的信息必须可单表或少量关联查出
- 第一版允许把论文图像直接存在 PostgreSQL 中
- PDF 文件本体建议保存在本地挂载目录，数据库保存完整元信息
- 配置账号、密码、IP 等不进数据库，按你的要求保存在本地 JSON 文件
- V1 不接入真实公众号发布接口，只做站内展示与人工审阅
- 文章主存储格式改为 Markdown，HTML 作为可选渲染结果

---

## 当前范围

V1 需要落库的内容包括：

- 论文基础信息
- arXiv 原始版本信息
- 评分结果
- PDF 下载与解析结果
- 结构化总结结果
- 全部提取图像的图片二进制与元信息
- 站内审核草稿
- Prompt 模板
- 系统配置
- 定时任务运行日志

V1 暂不强制要求：

- 多用户复杂权限模型
- 多租户
- 多公众号账号隔离
- 向量数据库
- 自动发布到微信公众号

---

## 表清单

V1 建议最小表集合：

- `papers`
- `paper_versions`
- `paper_scores`
- `paper_contents`
- `paper_assets`
- `article_drafts`
- `prompt_templates`
- `system_configs`
- `task_runs`

V1 可选扩展表：

- `publish_jobs`
- `review_logs`
- `publish_status_logs`
- `task_run_steps`
- `audit_logs`

---

## 实体关系

建议关系如下：

```text
papers 1 --- n paper_versions
papers 1 --- n paper_scores
papers 1 --- 1 paper_contents
papers 1 --- n paper_assets
papers 1 --- n article_drafts
```

说明：

- 一篇论文会有多个 arXiv 版本
- 一篇论文会有多次评分记录
- 一篇论文在 V1 通常只有一份当前解析内容，但可保留重跑更新
- 一篇论文会对应一个 PDF 资产和多张提取图像资产
- 一篇论文可生成多份草稿，但 V1 默认只有一个主草稿
- `publish_jobs` 作为 V2 预留表，不纳入 V1 主链路

---

## 状态枚举建议

### 论文状态 `paper_status`

建议使用受控字符串，不急着上 PostgreSQL ENUM，便于 V1 快速迭代：

- `DISCOVERED`
- `FILTERED`
- `SCORED`
- `RECOMMENDED`
- `PDF_DOWNLOADED`
- `PARSED`
- `CONTENT_GENERATED`
- `DRAFT_READY`
- `REVIEWING`
- `APPROVED`
- `ARCHIVED`
- `PUBLISH_PENDING`
- `PUBLISHING`
- `PUBLISHED`
- `FILTER_FAILED`
- `SCORE_FAILED`
- `PARSE_FAILED`
- `GENERATE_FAILED`
- `REVIEW_REJECTED`
- `PUBLISH_FAILED`

### 解析状态 `parse_status`

- `PENDING`
- `DOWNLOADING`
- `DOWNLOADED`
- `PARSING`
- `PARSED`
- `FAILED`

### 草稿审核状态 `review_status`

- `DRAFT`
- `REVIEWING`
- `APPROVED`
- `REJECTED`

### 发布状态 `publish_status`

- `PENDING`
- `SUBMITTED`
- `PROCESSING`
- `SUCCESS`
- `FAILED`
- `CANCELLED`

### 资源类型 `asset_type`

- `PDF`
- `FIGURE`

---

## 字段类型约定

推荐字段类型：

- 主键：`BIGSERIAL`
- 外键：`BIGINT`
- 短文本：`VARCHAR`
- 长文本：`TEXT`
- 枚举状态：`VARCHAR(32)`
- 结构化字段：`JSONB`
- 时间：`TIMESTAMPTZ`
- 布尔：`BOOLEAN`
- 数值评分：`NUMERIC(5,2)` 或 `INTEGER`
- 图片二进制：`BYTEA`

---

## 详细表设计

## 1. `papers`

### 作用

记录论文主实体，是所有业务链路的核心主表。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `arxiv_id` | `VARCHAR(64)` | `NOT NULL` `UNIQUE` | arXiv 论文 ID，不含版本号 |
| `latest_version_no` | `INTEGER` | `NOT NULL DEFAULT 1` | 当前最新版本号 |
| `title` | `TEXT` | `NOT NULL` | 论文标题 |
| `authors` | `JSONB` | `NOT NULL DEFAULT '[]'` | 作者列表 |
| `abstract` | `TEXT` | `NOT NULL` | 原始摘要 |
| `primary_category` | `VARCHAR(128)` | `NOT NULL` | 主分类 |
| `categories` | `JSONB` | `NOT NULL DEFAULT '[]'` | 分类列表 |
| `published_at` | `TIMESTAMPTZ` | `NOT NULL` | arXiv 首次发布时间 |
| `source_updated_at` | `TIMESTAMPTZ` | `NOT NULL` | arXiv 最近更新时间 |
| `pdf_url` | `TEXT` | `NOT NULL` | PDF 地址 |
| `source_url` | `TEXT` | `NOT NULL` | arXiv 详情页地址 |
| `paper_status` | `VARCHAR(32)` | `NOT NULL` | 主状态 |
| `is_candidate` | `BOOLEAN` | `NOT NULL DEFAULT TRUE` | 是否进入候选池 |
| `is_recommended` | `BOOLEAN` | `NOT NULL DEFAULT FALSE` | 是否被选为推荐论文 |
| `recommended_on` | `DATE` |  | 被选为推荐的业务日期 |
| `last_score_id` | `BIGINT` |  | 最近一次评分记录 ID |
| `last_content_id` | `BIGINT` |  | 最近一次解析内容 ID |
| `last_draft_id` | `BIGINT` |  | 最近一次草稿 ID |
| `failure_reason` | `TEXT` |  | 最近失败原因摘要 |
| `retry_count` | `INTEGER` | `NOT NULL DEFAULT 0` | 当前阶段累计重试次数 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 更新时间 |

### 必要索引

- `UNIQUE (arxiv_id)`
- `INDEX (paper_status)`
- `INDEX (recommended_on)`
- `INDEX (source_updated_at DESC)`
- `INDEX (is_recommended, recommended_on DESC)`

### `authors` JSONB 示例

```json
[
  { "name": "Author A", "affiliation": null },
  { "name": "Author B", "affiliation": null }
]
```

### `categories` JSONB 示例

```json
["cs.CV", "eess.IV", "cs.AI"]
```

---

## 2. `paper_versions`

### 作用

保存 arXiv 元数据版本快照，支持版本追踪和重放。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `paper_id` | `BIGINT` | `NOT NULL` FK | 关联 `papers.id` |
| `version_no` | `INTEGER` | `NOT NULL` | arXiv 版本号 |
| `title` | `TEXT` | `NOT NULL` | 当时版本标题 |
| `authors` | `JSONB` | `NOT NULL DEFAULT '[]'` | 当时作者信息 |
| `abstract` | `TEXT` | `NOT NULL` | 当时摘要 |
| `primary_category` | `VARCHAR(128)` | `NOT NULL` | 当时主分类 |
| `categories` | `JSONB` | `NOT NULL DEFAULT '[]'` | 当时分类列表 |
| `published_at` | `TIMESTAMPTZ` | `NOT NULL` | 版本发布时间 |
| `source_updated_at` | `TIMESTAMPTZ` | `NOT NULL` | arXiv feed 更新时间 |
| `pdf_url` | `TEXT` | `NOT NULL` | PDF 地址 |
| `source_payload` | `JSONB` | `NOT NULL` | arXiv 原始返回快照 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 入库时间 |

### 约束与索引

- `UNIQUE (paper_id, version_no)`
- `INDEX (paper_id, version_no DESC)`

---

## 3. `paper_scores`

### 作用

保存规则打分与 LLM 重排结果，支持回溯当天为什么推荐这篇论文。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `paper_id` | `BIGINT` | `NOT NULL` FK | 关联 `papers.id` |
| `score_date` | `DATE` | `NOT NULL` | 评分所属业务日期 |
| `topic_score` | `INTEGER` | `NOT NULL DEFAULT 0` | 主题相关度 |
| `foundation_model_score` | `INTEGER` | `NOT NULL DEFAULT 0` | 基础模型属性分 |
| `novelty_score` | `INTEGER` | `NOT NULL DEFAULT 0` | 技术新颖性分 |
| `practicality_score` | `INTEGER` | `NOT NULL DEFAULT 0` | 落地价值分 |
| `evidence_score` | `INTEGER` | `NOT NULL DEFAULT 0` | 证据完整性分 |
| `total_score` | `INTEGER` | `NOT NULL DEFAULT 0` | 总分 |
| `recommendation` | `VARCHAR(32)` | `NOT NULL` | 推荐等级，如 `high` |
| `score_reasons` | `JSONB` | `NOT NULL DEFAULT '[]'` | 推荐理由 |
| `risk_notes` | `JSONB` | `NOT NULL DEFAULT '[]'` | 风险提示 |
| `score_detail` | `JSONB` | `NOT NULL DEFAULT '{}'` | 评分细节 |
| `rank_in_day` | `INTEGER` |  | 当日排序 |
| `rule_version` | `VARCHAR(64)` | `NOT NULL` | 规则版本 |
| `model_name` | `VARCHAR(128)` |  | LLM 模型名称 |
| `prompt_version` | `VARCHAR(64)` | `NOT NULL` | Prompt 版本 |
| `raw_llm_response` | `JSONB` |  | 原始 LLM 结构化输出 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |

### 约束与索引

- `UNIQUE (paper_id, score_date, prompt_version)`
- `INDEX (score_date, total_score DESC)`
- `INDEX (paper_id, created_at DESC)`

### `score_detail` JSONB 示例

```json
{
  "keyword_hits": ["remote sensing", "foundation model"],
  "category_hits": ["cs.CV"],
  "has_code": false,
  "has_dataset": true,
  "top_n_pool": 10
}
```

---

## 4. `paper_contents`

### 作用

保存 PDF 下载、解析和结构化总结结果。这张表是“把论文转成可用内容”的核心表。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `paper_id` | `BIGINT` | `NOT NULL` FK | 关联 `papers.id` |
| `parse_status` | `VARCHAR(32)` | `NOT NULL` | 解析状态 |
| `pdf_local_path` | `TEXT` |  | 本地 PDF 路径 |
| `pdf_file_name` | `VARCHAR(512)` |  | PDF 文件名 |
| `pdf_size_bytes` | `BIGINT` |  | PDF 大小 |
| `pdf_sha256` | `VARCHAR(64)` |  | PDF 内容哈希 |
| `pdf_page_count` | `INTEGER` |  | 页数 |
| `pdf_metadata` | `JSONB` | `NOT NULL DEFAULT '{}'` | PDF 元信息 |
| `section_outline` | `JSONB` | `NOT NULL DEFAULT '[]'` | 章节大纲 |
| `parsed_sections` | `JSONB` | `NOT NULL DEFAULT '{}'` | 章节文本结构 |
| `abstract_cn` | `TEXT` |  | 中文摘要总结 |
| `innovations_cn` | `TEXT` |  | 创新点总结 |
| `methods_cn` | `TEXT` |  | 方法总结 |
| `experiments_cn` | `TEXT` |  | 实验总结 |
| `conclusion_cn` | `TEXT` |  | 结论总结 |
| `limitations_cn` | `TEXT` |  | 局限与未来工作 |
| `structured_summary` | `JSONB` | `NOT NULL DEFAULT '{}'` | 完整结构化总结 |
| `raw_parser_output` | `JSONB` | `NOT NULL DEFAULT '{}'` | 原始解析输出 |
| `raw_generation_output` | `JSONB` | `NOT NULL DEFAULT '{}'` | 原始总结生成输出 |
| `parser_version` | `VARCHAR(64)` | `NOT NULL` | 解析器版本 |
| `prompt_version` | `VARCHAR(64)` | `NOT NULL` | 总结 Prompt 版本 |
| `error_message` | `TEXT` |  | 失败错误摘要 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 更新时间 |

### 约束与索引

- `UNIQUE (paper_id)`
- `INDEX (parse_status)`

### `structured_summary` JSONB 建议格式

这个字段要能够承载你给的那种论文总结结构。

```json
{
  "paper_title": "SAM-MI: A Mask-Injected Framework...",
  "paper_venue": "arXiv",
  "paper_code_url": null,
  "summary_sections": {
    "abstract": "......",
    "innovations": [
      {
        "name": "文本引导的稀疏点提示器",
        "problem": "计算开销过大",
        "mechanism": "基于像素-文本代价图进行稀疏采样",
        "effect": "提示点数量减少 96.0%"
      }
    ],
    "method": "......",
    "experiments": "......",
    "conclusion": "......",
    "limitations": "......"
  },
  "key_metrics": [
    {
      "name": "mIoU relative improvement",
      "value": "16.7%"
    },
    {
      "name": "speedup",
      "value": "1.6x"
    }
  ]
}
```

### `parsed_sections` JSONB 建议格式

```json
{
  "abstract": "original abstract text",
  "introduction": "section text",
  "method": "section text",
  "experiment": "section text",
  "conclusion": "section text"
}
```

### `pdf_metadata` JSONB 建议格式

```json
{
  "title": "paper title from pdf metadata",
  "author": "authors from metadata",
  "producer": "pdf producer",
  "creator": "pdf creator",
  "creation_date": "2026-04-10T00:00:00Z"
}
```

---

## 5. `paper_assets`

### 作用

统一记录 PDF 和图像资产，满足“保存全部可提取图像，并可在实验部分展示”的需求。

### 设计取舍

V1 允许图像直接存数据库 `BYTEA`，原因是：

- 每天只处理极少量论文
- V1 数据量可控
- 便于快速做前端预览和后续公众号素材处理

但需要明确：

- 图像长期大量入库会让 PostgreSQL 体积膨胀
- 如果后续规模上来，应迁移到对象存储或本地文件系统 + 元信息入库

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `paper_id` | `BIGINT` | `NOT NULL` FK | 关联 `papers.id` |
| `asset_type` | `VARCHAR(16)` | `NOT NULL` | `PDF` 或 `FIGURE` |
| `asset_role` | `VARCHAR(64)` | `NOT NULL` | 如 `original_pdf`、`figure_1` |
| `source_url` | `TEXT` |  | 来源地址 |
| `local_path` | `TEXT` |  | 本地文件路径 |
| `file_name` | `VARCHAR(512)` |  | 文件名 |
| `mime_type` | `VARCHAR(128)` |  | MIME 类型 |
| `size_bytes` | `BIGINT` |  | 文件大小 |
| `sha256` | `VARCHAR(64)` |  | 文件哈希 |
| `page_no` | `INTEGER` |  | 来源页码 |
| `figure_index` | `INTEGER` |  | 图序号，按解析顺序递增 |
| `caption` | `TEXT` |  | 图注 |
| `width` | `INTEGER` |  | 图片宽度 |
| `height` | `INTEGER` |  | 图片高度 |
| `display_order` | `INTEGER` |  | 站内展示顺序 |
| `is_experiment_figure` | `BOOLEAN` | `NOT NULL DEFAULT FALSE` | 是否归入实验部分展示 |
| `binary_data` | `BYTEA` |  | 图像二进制，PDF 默认可为空 |
| `extra_metadata` | `JSONB` | `NOT NULL DEFAULT '{}'` | 补充信息 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |

### 约束与索引

- `INDEX (paper_id, asset_type)`
- `UNIQUE (paper_id, asset_role)`
- `UNIQUE (paper_id, figure_index) WHERE figure_index IS NOT NULL`
- `INDEX (paper_id, is_experiment_figure, display_order)`

### 保存策略

- `PDF` 资产：记录完整元信息，文件本体优先保存在本地挂载目录
- `FIGURE` 资产：保存全部可提取图像，并将二进制保存到 `binary_data`
- 图像默认按 `figure_index` 排序，后续可通过规则或人工将部分图归入实验部分

### `extra_metadata` JSONB 示例

```json
{
  "extractor": "pdf-figure-extractor-v1",
  "source_page_label": "Page 3",
  "confidence": 0.92
}
```

---

## 6. `article_drafts`

### 作用

保存站内文章草稿和人工审核后的结果。V1 以 Markdown 为主格式。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `paper_id` | `BIGINT` | `NOT NULL` FK | 关联 `papers.id` |
| `draft_version` | `INTEGER` | `NOT NULL DEFAULT 1` | 草稿版本 |
| `is_primary` | `BOOLEAN` | `NOT NULL DEFAULT TRUE` | 是否主草稿 |
| `title` | `TEXT` | `NOT NULL` | 推荐标题 |
| `alt_titles` | `JSONB` | `NOT NULL DEFAULT '[]'` | 备选标题 |
| `summary` | `TEXT` |  | 摘要 |
| `intro_text` | `TEXT` |  | 导语 |
| `markdown_content` | `TEXT` | `NOT NULL` | 主稿 Markdown 内容 |
| `rendered_html` | `TEXT` |  | 渲染后的 HTML，用于站内展示 |
| `cover_text` | `TEXT` |  | 封面文案 |
| `tags` | `JSONB` | `NOT NULL DEFAULT '[]'` | 标签 |
| `review_status` | `VARCHAR(32)` | `NOT NULL DEFAULT 'DRAFT'` | 审核状态 |
| `reviewer_id` | `BIGINT` |  | 审核人 ID，V1 保留字段但不接真实用户表 |
| `review_comment` | `TEXT` |  | 审核意见 |
| `approved_at` | `TIMESTAMPTZ` |  | 审核通过时间 |
| `template_version` | `VARCHAR(64)` | `NOT NULL` | 写作模板版本 |
| `prompt_version` | `VARCHAR(64)` | `NOT NULL` | 写作 Prompt 版本 |
| `source_content_id` | `BIGINT` |  | 关联 `paper_contents.id` |
| `site_slug` | `VARCHAR(256)` |  | 站内页面 slug |
| `site_path` | `TEXT` |  | 站内访问路径 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 更新时间 |

### 约束与索引

- `INDEX (paper_id, created_at DESC)`
- `INDEX (review_status, created_at DESC)`
- `UNIQUE (paper_id, draft_version)`
- `UNIQUE (site_slug) WHERE site_slug IS NOT NULL`

---

## 7. `publish_jobs`

### 作用

记录公众号发布任务和回查结果。该表属于 V2 预留，V1 可以不建。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `draft_id` | `BIGINT` | `NOT NULL` FK | 关联 `article_drafts.id` |
| `channel` | `VARCHAR(32)` | `NOT NULL DEFAULT 'wechat'` | 发布渠道 |
| `draft_media_id` | `VARCHAR(128)` |  | 微信草稿 media_id |
| `publish_id` | `VARCHAR(128)` |  | 微信 publish_id |
| `publish_status` | `VARCHAR(32)` | `NOT NULL DEFAULT 'PENDING'` | 发布状态 |
| `retry_count` | `INTEGER` | `NOT NULL DEFAULT 0` | 重试次数 |
| `wechat_errcode` | `INTEGER` |  | 微信错误码 |
| `wechat_errmsg` | `TEXT` |  | 微信错误消息 |
| `publish_response` | `JSONB` | `NOT NULL DEFAULT '{}'` | 原始返回 |
| `failed_reason` | `TEXT` |  | 失败原因 |
| `scheduled_publish_at` | `TIMESTAMPTZ` |  | 预约发布时间 |
| `submitted_at` | `TIMESTAMPTZ` |  | 提交发布时间 |
| `published_at` | `TIMESTAMPTZ` |  | 实际发布时间 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 更新时间 |

### 约束与索引

- `INDEX (draft_id, created_at DESC)`
- `INDEX (publish_status, created_at DESC)`
- `UNIQUE (draft_id, channel, created_at)`

说明：

- 严格的“一个草稿一次发布”可以通过服务层控制
- 数据库层不强行限制只有一条记录，方便保留重试和重发历史

---

## 8. `prompt_templates`

### 作用

保存评分、总结、写作等 Prompt 模板。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `template_type` | `VARCHAR(64)` | `NOT NULL` | 如 `ranking`、`summary`、`article` |
| `template_name` | `VARCHAR(128)` | `NOT NULL` | 模板名称 |
| `template_content` | `TEXT` | `NOT NULL` | 模板正文 |
| `template_variables` | `JSONB` | `NOT NULL DEFAULT '[]'` | 变量列表 |
| `version` | `VARCHAR(64)` | `NOT NULL` | 版本号 |
| `is_active` | `BOOLEAN` | `NOT NULL DEFAULT TRUE` | 是否启用 |
| `remark` | `TEXT` |  | 备注 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 更新时间 |

### 约束与索引

- `UNIQUE (template_type, version)`
- `INDEX (template_type, is_active)`

---

## 9. `system_configs`

### 作用

保存系统级配置。注意：敏感账号密码按你的要求放本地 JSON，不建议存这里。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `config_key` | `VARCHAR(128)` | `NOT NULL` `UNIQUE` | 配置键 |
| `config_value` | `JSONB` | `NOT NULL` | 配置值 |
| `description` | `TEXT` |  | 描述 |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 更新时间 |

### 推荐配置项

- 关键词列表
- 分类白名单
- 定时抓取窗口
- 每日推荐篇数
- Top N 数量
- Prompt 默认版本
- 站内展示开关与默认模板

---

## 10. `task_runs`

### 作用

记录每日任务和重跑任务执行情况。

### 建议字段

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | `BIGSERIAL` | PK | 主键 |
| `task_type` | `VARCHAR(64)` | `NOT NULL` | 任务类型 |
| `biz_date` | `DATE` | `NOT NULL` | 业务日期 |
| `status` | `VARCHAR(32)` | `NOT NULL` | 任务状态 |
| `trigger_source` | `VARCHAR(32)` | `NOT NULL` | `scheduler` 或 `manual` |
| `started_at` | `TIMESTAMPTZ` | `NOT NULL` | 开始时间 |
| `ended_at` | `TIMESTAMPTZ` |  | 结束时间 |
| `duration_ms` | `BIGINT` |  | 耗时 |
| `result_summary` | `JSONB` | `NOT NULL DEFAULT '{}'` | 结果摘要 |
| `error_message` | `TEXT` |  | 错误信息 |
| `retry_count` | `INTEGER` | `NOT NULL DEFAULT 0` | 重试次数 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | 创建时间 |

### 约束与索引

- `INDEX (task_type, biz_date DESC)`
- `INDEX (status, started_at DESC)`

---

## 哪些字段必须使用 JSONB

当前建议必须用 `JSONB` 的字段：

- `papers.authors`
- `papers.categories`
- `paper_versions.authors`
- `paper_versions.categories`
- `paper_versions.source_payload`
- `paper_scores.score_reasons`
- `paper_scores.risk_notes`
- `paper_scores.score_detail`
- `paper_scores.raw_llm_response`
- `paper_contents.pdf_metadata`
- `paper_contents.section_outline`
- `paper_contents.parsed_sections`
- `paper_contents.structured_summary`
- `paper_contents.raw_parser_output`
- `paper_contents.raw_generation_output`
- `paper_assets.extra_metadata`
- `article_drafts.alt_titles`
- `article_drafts.tags`
- `prompt_templates.template_variables`
- `system_configs.config_value`
- `task_runs.result_summary`

原因很简单：

- 字段结构未来会演进
- 需要保留原始结构化结果
- 后续前端会直接消费部分 JSON 结构

---

## 幂等与唯一约束

### 核心幂等规则

- 同一 `arxiv_id` 不重复建论文主记录
- 同一 `paper_id + version_no` 不重复插版本
- 同一 `paper_id + score_date + prompt_version` 不重复插评分
- 同一 `paper_id` 在 `paper_contents` 中只有一条当前解析记录
- 同一 `paper_id + figure_index` 最多保留一张图
- 同一 `paper_id + draft_version` 不重复建草稿版本

### 服务层需额外控制

- 默认配置下同一天只允许固定数量的论文进入推荐集合，初始值为 1，实际数量从 `system_configs` 读取
- 未审核通过的草稿不能进入后续发布链路
- 如果未来启用发布，发布失败重试应复用同一业务上下文，不重新生成新论文记录

---

## 查询场景与索引考虑

V1 高频查询大概只有这几类：

### 场景 1：查当天推荐论文

依赖：

- `papers.is_recommended`
- `papers.recommended_on`

### 场景 2：查论文详情页

依赖：

- `papers`
- `paper_scores`
- `paper_contents`
- `paper_assets`

### 场景 3：查待审核草稿

依赖：

- `article_drafts.review_status`
- `article_drafts.created_at`

### 场景 4：查站内待审稿件

依赖：

- `article_drafts.review_status`
- `article_drafts.updated_at`

---

## 是否需要软删除

当前建议：

- V1 不做通用软删除字段
- 采用“状态失效”或“逻辑作废”即可

原因：

- 当前是内部工作流系统
- 软删除会增加每个查询的复杂度
- 真正需要保留审计时，更适合保留状态和日志，而不是删除

---

## 是否需要用户表

当前建议：

- V1 可不实现完整用户体系
- 当前前端只给单人使用，`reviewer_id` 先保留但不加外键
- 如果下一步要做登录，再补 `users` 和 `roles`

换句话说，现在数据库先围绕论文工作流建模，不被用户系统阻塞。

---

## 本地 JSON 配置文件约定

这部分不属于数据库表，但你已经明确要求“IP、密码、账号等保存在 JSON 文件中”，所以这里一起约定。

### 建议文件

- `configs/runtime.local.json`

### 用途

保存本地 Docker 环境和第三方接入配置，例如：

- PostgreSQL 地址
- Redis 地址
- LLM 接口配置
- 本地挂载目录
- 站内服务地址

### 建议结构

```json
{
  "postgres": {
    "host": "127.0.0.1",
    "port": 5432,
    "user": "postgres",
    "password": "postgres",
    "database": "arxiv_agent"
  },
  "redis": {
    "host": "127.0.0.1",
    "port": 6379,
    "password": ""
  },
  "storage": {
    "pdf_dir": "./data/pdfs",
    "image_dir": "./data/images"
  },
  "site": {
    "base_url": "http://127.0.0.1:3000",
    "draft_path_prefix": "/drafts"
  },
  "llm": {
    "base_url": "",
    "api_key": "",
    "model": ""
  }
}
```

### 注意事项

- 这个文件只适合本地开发
- 应加入 `.gitignore`
- 后续如果部署到正式环境，建议迁移到环境变量或密钥服务

---

## 基于你当前需求的数据库结论

结合你已经给出的要求，第一阶段数据库必须满足以下能力：

1. 能记录每天抓到的论文主信息
2. 能记录论文版本和原始 arXiv 响应
3. 能记录评分并选出当天最优 1 篇
4. 能记录 PDF 的完整元信息和本地路径
5. 能把全部提取图像存进数据库并支持实验部分展示
6. 能保存你示例那种结构化中文总结
7. 能生成 Markdown 草稿并保留审核状态
8. 能给站内页面渲染和人工审阅提供数据

---

## 下一步建议

基于这份数据库设计，下一步最适合继续做的是：

1. 输出 PostgreSQL 建表 SQL 初稿
2. 输出 Go 后端的 `model` 和 `repository` 结构
3. 输出 `configs/runtime.local.json.example`
4. 继续细化 `docs/04-api-design.md`
