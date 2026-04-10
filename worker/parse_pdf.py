import argparse
import json
import os
import re
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
        return {"full_text": short_text[:20000]}, [{"title": "full_text"}]

    sections = {}
    outline = []
    for i, (line_idx, title) in enumerate(matches):
        end_idx = matches[i + 1][0] if i + 1 < len(matches) else len(lines)
        content = "\n".join(lines[line_idx + 1 : end_idx]).strip()
        key = title
        if key in sections:
            key = f"{key}_{i+1}"
        sections[key] = normalize_space(content)
        outline.append({"title": key})
    return sections, outline


def first_sentences(text: str, max_chars: int = 800) -> str:
    text = normalize_space(text)
    if len(text) <= max_chars:
        return text
    clipped = text[:max_chars]
    for sep in [". ", "; ", ": "]:
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
    return ""


def extract_images(doc: fitz.Document, image_dir: Path):
    image_dir.mkdir(parents=True, exist_ok=True)
    figures = []
    seen = set()
    figure_index = 1
    for page_no in range(len(doc)):
        page = doc.load_page(page_no)
        page_text = page.get_text("text")
        for image_info in page.get_images(full=True):
            xref = image_info[0]
            if xref in seen:
                continue
            seen.add(xref)
            try:
                image = doc.extract_image(xref)
            except Exception:
                continue
            ext = image.get("ext", "png")
            image_bytes = image.get("image")
            if not image_bytes:
                continue
            file_name = f"figure_{figure_index}.{ext}"
            file_path = image_dir / file_name
            file_path.write_bytes(image_bytes)
            figures.append(
                {
                    "figure_index": figure_index,
                    "page_no": page_no + 1,
                    "file_name": file_name,
                    "local_path": str(file_path),
                    "mime_type": f"image/{ext}",
                    "size_bytes": len(image_bytes),
                    "caption": detect_caption(page_text, figure_index),
                }
            )
            figure_index += 1
    return figures


def main():
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

    abstract_text = sections.get("abstract", "") or first_sentences(full_text, 1200)
    method_text = ""
    for key in ["method", "methods", "methodology", "approach"]:
        if sections.get(key):
            method_text = sections[key]
            break
    experiment_text = ""
    for key in ["experiment", "experiments", "experimental setup", "results"]:
        if sections.get(key):
            experiment_text = sections[key]
            break
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
            "abstract": first_sentences(abstract_text, 1400),
            "innovations": first_sentences(method_text, 1200),
            "method": first_sentences(method_text, 1800),
            "experiments": first_sentences(experiment_text, 1800),
            "conclusion": first_sentences(conclusion_text, 1200),
            "limitations": first_sentences(limitations_text, 1200),
        },
        "figures": figures,
        "raw_text_excerpt": first_sentences(full_text, 4000),
    }
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()

