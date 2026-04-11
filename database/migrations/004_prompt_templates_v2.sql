UPDATE prompt_templates
SET is_active = FALSE
WHERE template_type = 'summary';

INSERT INTO prompt_templates (
    template_type,
    template_name,
    template_content,
    template_variables,
    version,
    is_active,
    remark
) VALUES (
    'summary',
    'llm-summary-v2',
    '你是一个严谨的遥感与计算机视觉论文编辑，请基于给定论文信息生成一版详细中文审阅稿。要求：
1. 只能依据给定内容总结，不得编造论文中不存在的事实、实验、公式或代码地址。
2. 输出必须是 JSON，不要输出额外说明，不要使用 Markdown 代码块包裹 JSON。
3. markdown_content 必须是完整 Markdown 长文，长度尽量充实，不能只是简短摘要。
4. 正文结构尽量采用以下顺序：
   论文标题、论文期刊、论文代码、
   1 摘要、
   2 创新点、
   3 方法、
   4 实验、
   5 结论、
   6 局限性与未来工作。
5. 如果给定了流程图/主图候选，请优先在“创新点”或“方法”部分引用它们；如果给定了公式候选，请优先在“方法”中整理为关键公式块。
6. 如果无法可靠恢复公式或代码地址，明确写“无”或“建议人工补充”，不要编造。

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
论文代码：{{paper_code_url}}
论文摘要：{{paper_abstract}}
章节摘要：{{section_summaries}}
章节原文摘录：{{parsed_sections}}
流程图/主图候选：{{selected_figures}}
图像图注：{{figure_captions}}
公式候选：{{equations}}',
    '["paper_title","arxiv_id","source_url","paper_code_url","paper_abstract","section_summaries","parsed_sections","selected_figures","figure_captions","equations"]'::jsonb,
    'llm-summary-v2',
    TRUE,
    '详细长文模板，强调流程图、公式和局限性'
)
ON CONFLICT (template_type, version) DO UPDATE
SET template_name = EXCLUDED.template_name,
    template_content = EXCLUDED.template_content,
    template_variables = EXCLUDED.template_variables,
    is_active = EXCLUDED.is_active,
    remark = EXCLUDED.remark,
    updated_at = NOW();
