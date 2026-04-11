import argparse
import json
import re
import sys
from pathlib import Path

import fitz


SECTION_NAMES = [
    "abstract",
    "introduction",
    "related work",
    "background",
    "method",
    "methods",
    "methodology",
    "approach",
    "experiment",
    "experiments",
    "experimental setup",
    "results",
    "discussion",
    "conclusion",
    "limitations",
]

DIAGRAM_KEYWORDS = [
    "framework",
    "pipeline",
    "overview",
    "architecture",
    "workflow",
    "method",
    "approach",
    "module",
    "network",
    "system",
    "training",
    "design",
    "overall",
]

RESULT_KEYWORDS = [
    "ablation",
    "comparison",
    "quantitative",
    "qualitative",
    "benchmark",
    "results",
    "cityscapes",
    "ade20k",
    "coco",
    "voc",
]

MATH_TOKENS = ["=", "∑", "∈", "λ", "α", "β", "γ", "δ", "θ", "μ", "σ", "||", "log(", "exp(", "argmax", "softmax"]


def normalize_space(text: str) -> str:
    return re.sub(r"\s+", " ", (text or "").strip())


def split_sections(full_text: str):
    lines = [line.rstrip() for line in full_text.splitlines()]
    matches = []
    pattern = re.compile(
        r"^\s*(?:\d+(?:\.\d+)*)?\s*(%s)\s*$" % "|".join(re.escape(name) for name in SECTION_NAMES),
        re.IGNORECASE,
    )
    for idx, line in enumerate(lines):
        if pattern.match(line.strip()):
            matches.append((idx, normalize_space(line.lower())))

    if not matches:
        short_text = normalize_space(full_text)
        return {"full_text": short_text[:30000]}, [{"title": "full_text"}]

    sections = {}
    outline = []
    for i, (line_idx, title) in enumerate(matches):
        end_idx = matches[i + 1][0] if i + 1 < len(matches) else len(lines)
        content = "\n".join(lines[line_idx + 1 : end_idx]).strip()
        key = title
        if key in sections:
            key = f"{key}_{i + 1}"
        sections[key] = normalize_space(content)
        outline.append({"title": key})
    return sections, outline


def clip_text(text: str, max_chars: int = 1200) -> str:
    text = normalize_space(text)
    if len(text) <= max_chars:
        return text
    clipped = text[:max_chars]
    for sep in ["。", ". ", "; ", ": ", "! ", "? "]:
        pos = clipped.rfind(sep)
        if pos > max_chars // 2:
            return clipped[: pos + 1].strip()
    return clipped.strip()


def detect_caption(page_text: str, figure_index: int) -> str:
    patterns = [
        rf"(Figure|Fig\.?)\s*{figure_index}\s*[:.\-]?\s*(.+)",
        rf"(Figure|Fig\.?)\s*{figure_index + 1}\s*[:.\-]?\s*(.+)",
    ]
    for pattern in patterns:
        match = re.search(pattern, page_text, re.IGNORECASE)
        if match:
            return normalize_space(match.group(2))[:500]

    generic = re.search(r"(Figure|Fig\.?)\s*\d+\s*[:.\-]?\s*(.+)", page_text, re.IGNORECASE)
    if generic:
        return normalize_space(generic.group(2))[:500]
    return ""


def score_figure(caption: str, page_no: int, width: int, height: int, area_ratio: float) -> float:
    score = 0.0
    caption_lower = caption.lower()

    if caption:
        score += 2.0
    if any(keyword in caption_lower for keyword in DIAGRAM_KEYWORDS):
        score += 5.0
    if any(keyword in caption_lower for keyword in ["framework", "pipeline", "architecture", "workflow", "overview"]):
        score += 3.0
    if any(keyword in caption_lower for keyword in RESULT_KEYWORDS):
        score -= 5.0

    if area_ratio >= 0.18:
        score += 4.0
    elif area_ratio >= 0.10:
        score += 3.0
    elif area_ratio >= 0.05:
        score += 1.5

    if width >= 700 and height >= 350:
        score += 1.5
    elif width >= 500 and height >= 250:
        score += 1.0

    if page_no <= 4:
        score += 1.5

    return round(score, 2)


def select_primary_figures(candidates):
    if not candidates:
        return []

    deduped = []
    seen_caption_keys = {}
    for item in candidates:
        caption_key = (item["page_no"], normalize_space(item["caption"]).lower())
        if caption_key[1]:
            existing = seen_caption_keys.get(caption_key)
            if existing is None or item["size_bytes"] > existing["size_bytes"]:
                seen_caption_keys[caption_key] = item
            continue
        deduped.append(item)

    deduped.extend(seen_caption_keys.values())

    ranked = sorted(
        deduped,
        key=lambda item: (item["diagram_score"], item["size_bytes"], -item["page_no"]),
        reverse=True,
    )

    selected = [item for item in ranked if item["diagram_score"] >= 4.0][:6]
    if not selected:
        selected = ranked[:4]

    selected_ids = {item["figure_index"] for item in selected}
    filtered = [item for item in deduped if item["figure_index"] in selected_ids]
    filtered.sort(key=lambda item: (-item["diagram_score"], item["page_no"], item["figure_index"]))
    return filtered


def extract_images(doc: fitz.Document, image_dir: Path):
    image_dir.mkdir(parents=True, exist_ok=True)
    candidates = []
    seen = set()
    figure_index = 1

    for page_no in range(len(doc)):
        page = doc.load_page(page_no)
        page_text = page.get_text("text")
        page_area = max(float(page.rect.width * page.rect.height), 1.0)

        for image_info in page.get_images(full=True):
            xref = image_info[0]
            if xref in seen:
                continue
            seen.add(xref)

            try:
                image = doc.extract_image(xref)
            except Exception:
                continue

            image_bytes = image.get("image")
            width = int(image.get("width") or 0)
            height = int(image.get("height") or 0)
            if not image_bytes or width < 220 or height < 160 or len(image_bytes) < 12000:
                continue

            rects = page.get_image_rects(xref)
            rect_area = 0.0
            for rect in rects:
                rect_area = max(rect_area, float(rect.width * rect.height))
            area_ratio = rect_area / page_area if rect_area > 0 else 0.0

            ext = image.get("ext", "png")
            file_name = f"figure_{figure_index}.{ext}"
            file_path = image_dir / file_name
            file_path.write_bytes(image_bytes)

            caption = detect_caption(page_text, figure_index)
            candidates.append(
                {
                    "figure_index": figure_index,
                    "page_no": page_no + 1,
                    "file_name": file_name,
                    "local_path": str(file_path),
                    "mime_type": f"image/{ext}",
                    "size_bytes": len(image_bytes),
                    "caption": caption,
                    "width": width,
                    "height": height,
                    "diagram_score": score_figure(caption, page_no + 1, width, height, area_ratio),
                }
            )
            figure_index += 1

    return select_primary_figures(candidates)


def extract_equations(page_texts):
    results = []
    seen = set()

    for page_text in page_texts:
        for raw_line in page_text.splitlines():
            line = normalize_space(raw_line)
            if len(line) < 8 or len(line) > 220:
                continue

            lowered = line.lower()
            if lowered.startswith(("figure", "fig.", "table", "algorithm", "references")):
                continue
            if "=" not in line:
                continue
            if not re.search(r"[A-Za-z]\s*=\s*|\\mathcal|\\lambda|∑|∈|λ|α|β|γ|δ|θ|μ|σ|\|\||log\(|exp\(|argmax|softmax", line):
                continue
            if len(re.findall(r"[A-Za-z]", line)) < 2:
                continue
            if line.count(" ") > 28:
                continue

            dedupe_key = lowered
            if dedupe_key in seen:
                continue
            seen.add(dedupe_key)
            results.append(line)
            if len(results) >= 8:
                return results

    return results


def pick_section(sections, keys):
    for key in keys:
        if sections.get(key):
            return sections[key]
    return ""


def main():
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")

    parser = argparse.ArgumentParser()
    parser.add_argument("--pdf", required=True)
    parser.add_argument("--output-images-dir", required=True)
    parser.add_argument("--paper-title", required=True)
    parser.add_argument("--arxiv-id", required=True)
    args = parser.parse_args()

    pdf_path = Path(args.pdf)
    image_dir = Path(args.output_images_dir)

    doc = fitz.open(pdf_path)
    metadata = doc.metadata or {}
    page_texts = []
    for page_no in range(len(doc)):
        page = doc.load_page(page_no)
        page_texts.append(page.get_text("text"))

    full_text = "\n".join(page_texts)
    sections, outline = split_sections(full_text)
    figures = extract_images(doc, image_dir)
    equations = extract_equations(page_texts)

    abstract_text = sections.get("abstract", "") or clip_text(full_text, 1800)
    method_text = pick_section(sections, ["method", "methods", "methodology", "approach"])
    experiment_text = pick_section(sections, ["experiment", "experiments", "experimental setup", "results"])
    conclusion_text = sections.get("conclusion", "")
    limitations_text = sections.get("limitations", "")

    result = {
        "paper_title": args.paper_title,
        "arxiv_id": args.arxiv_id,
        "pdf_metadata": metadata,
        "pdf_page_count": len(doc),
        "section_outline": outline,
        "parsed_sections": sections,
        "summary": {
            "abstract": clip_text(abstract_text, 2200),
            "innovations": clip_text(method_text, 2200),
            "method": clip_text(method_text, 4200),
            "experiments": clip_text(experiment_text, 4200),
            "conclusion": clip_text(conclusion_text, 1800),
            "limitations": clip_text(limitations_text, 1800),
        },
        "figures": figures,
        "equations": equations,
        "raw_text_excerpt": clip_text(full_text, 8000),
    }
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
