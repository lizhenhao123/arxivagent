# 07 PDF Parsing And Generation

## 目标

定义 PDF 下载、内容解析、结构化理解和公众号文章生成规则，作为 Python Worker 与生成模块实现依据。

## 输入

- 推荐论文元数据
- PDF 地址
- Prompt 模板
- 风格模板

## 输出

- 结构化审稿 JSON
- Markdown 正文
- 可选渲染 HTML
- 推荐标题
- 摘要
- 封面文案
- 标签
- 图像清单与图注

## 解析目标

最低要求提取：

- 摘要
- 方法概述
- 实验设置
- 结果摘要
- 亮点
- 局限
- 全部可提取图像
- 图注文本

## 审稿 JSON 建议字段

- `paper_title`
- `arxiv_id`
- `problem_statement`
- `method_summary`
- `experiment_summary`
- `highlights`
- `limitations`
- `publish_recommendation`
- `risk_notes`

## 站内文章内容结构

1. 标题
2. 导语
3. 论文信息卡
4. 为什么值得关注
5. 方法解读
6. 实验结果解读
7. 实验相关图像与图注
7. 亮点与局限
8. 价值判断
9. 原文链接

## 写作要求

- 以总结和解读为主
- 不长段复制原文
- 显式区分客观事实与系统判断
- 主输出为可编辑 Markdown
- 可根据 Markdown 渲染站内 HTML
- 图像内容优先归入实验部分展示

## 与代码生成的关系

本文件会直接影响：

- Python PDF parser
- 结构化 schema
- LLM prompt 设计
- Markdown generator
- HTML renderer

## 当前结论

- PDF 解析至少要保留正文文本与图像资产
- 需要抽取图表标题
- 文章最终先服务于站内审阅，不直接接公众号

## 需要你补充的信息

- PDF 解析优先“正文文本”还是“版面结构”
- 是否需要术语表
- 最终文风偏研究解读还是运营科普
