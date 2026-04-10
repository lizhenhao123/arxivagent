INSERT INTO prompt_templates (
    template_type,
    template_name,
    template_content,
    template_variables,
    version,
    is_active,
    remark
) VALUES
(
    'summary',
    'llm-summary-v1',
    '你是一个严谨的遥感与计算机视觉研究助理。请基于给定论文信息生成结构化中文总结。要求：
1. 只根据提供内容总结，不要虚构。
2. 保持专业、克制、适合内部审阅。
3. 输出必须是 JSON，不要输出额外说明。
4. markdown_content 必须是完整 Markdown 草稿。

请输出如下 JSON 字段：
{
  "recommended_title": "string",
  "alt_titles": ["string", "string", "string"],
  "summary": "string",
  "intro_text": "string",
  "cover_text": "string",
  "tags": ["string"],
  "abstract_cn": "string",
  "innovations_cn": "string",
  "methods_cn": "string",
  "experiments_cn": "string",
  "conclusion_cn": "string",
  "limitations_cn": "string",
  "markdown_content": "string"
}

论文标题：{{paper_title}}
arXiv ID：{{arxiv_id}}
原文链接：{{source_url}}
论文摘要：{{paper_abstract}}
章节摘要：{{section_summaries}}
图像图注：{{figure_captions}}',
    '["paper_title","arxiv_id","source_url","paper_abstract","section_summaries","figure_captions"]'::jsonb,
    'llm-summary-v1',
    TRUE,
    '默认 LLM 结构化总结模板'
),
(
    'title',
    'llm-title-v1',
    '你是一个论文内容编辑。请基于给定论文摘要和方法亮点，为中文站内稿件拟定标题。输出 JSON：
{
  "recommended_title": "string",
  "alt_titles": ["string", "string", "string"]
}

标题：{{paper_title}}
摘要：{{paper_abstract}}
方法亮点：{{method_summary}}',
    '["paper_title","paper_abstract","method_summary"]'::jsonb,
    'llm-title-v1',
    FALSE,
    '备用标题模板'
)
ON CONFLICT (template_type, version) DO NOTHING;

