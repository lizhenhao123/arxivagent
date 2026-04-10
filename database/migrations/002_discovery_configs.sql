INSERT INTO system_configs (config_key, config_value, description) VALUES
    (
        'discovery_keywords',
        '{
          "topic_keywords": [
            "remote sensing",
            "earth observation",
            "satellite",
            "aerial",
            "geospatial",
            "sar",
            "multispectral",
            "hyperspectral",
            "change detection"
          ],
          "foundation_keywords": [
            "foundation model",
            "pretraining",
            "pre-training",
            "generalist",
            "vision-language",
            "multimodal",
            "vlm",
            "large-scale"
          ],
          "novelty_keywords": [
            "novel",
            "new",
            "framework",
            "benchmark",
            "dataset",
            "unified"
          ],
          "practicality_keywords": [
            "classification",
            "detection",
            "segmentation",
            "change detection",
            "localization",
            "retrieval"
          ],
          "evidence_keywords": [
            "experiment",
            "benchmark",
            "ablation",
            "baseline",
            "state-of-the-art",
            "code"
          ]
        }'::jsonb,
        '规则打分关键词'
    ),
    (
        'discovery_filters',
        '{
          "category_whitelist": [
            "cs.CV",
            "cs.AI",
            "eess.IV"
          ],
          "max_results": 50,
          "time_window_hours": 24
        }'::jsonb,
        '论文发现过滤参数'
    )
ON CONFLICT (config_key) DO NOTHING;

