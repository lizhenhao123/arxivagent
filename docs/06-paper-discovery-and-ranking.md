# 06 Paper Discovery And Ranking

## 目标

定义论文发现、过滤、打分、重排和推荐输出规则，作为抓取与推荐模块的实现依据。

## 数据源

唯一数据源：

- arXiv API

## 发现规则

- 按最近 24 小时拉取
- 按关键词和分类做第一轮过滤
- 对同一论文版本做幂等去重

## 推荐关键词方向

- 遥感：`remote sensing`、`earth observation`、`satellite`
- 空天与地理：`aerial`、`geospatial`
- 多模态：`multimodal`、`vision-language`
- 基础模型：`foundation model`、`pretraining`
- 特殊任务：`SAR`、`multispectral`、`hyperspectral`、`change detection`

## 评分框架

总分 100：

- 主题相关度：40
- 基础模型属性：25
- 技术新颖性：15
- 落地价值：10
- 证据完整性：10

## 双层筛选机制

### 第一层：规则筛选

- 标题关键词命中
- 摘要关键词命中
- 分类白名单命中
- 主题去重

### 第二层：LLM 重排

- 输出总分
- 输出推荐等级
- 输出理由和风险

## 输出结果

每次跑批建议输出：

- 今日推荐 1 篇
- 备选论文若干
- 评分明细
- 风险备注

## 与代码生成的关系

本文件会直接影响：

- arXiv client
- 过滤器实现
- 打分模块
- LLM prompt 设计
- 每日推荐任务逻辑

## 需要你补充的信息

- 分类白名单的精确列表
- Top N 的具体数量
- 是否要加入“作者/机构”加权
- 是否允许人工置顶某篇论文

