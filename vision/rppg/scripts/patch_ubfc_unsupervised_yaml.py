#!/usr/bin/env python3
"""
Patch rPPG-Toolbox configs/infer_configs/UBFC-rPPG_UNSUPERVISED.yaml in place:
  UNSUPERVISED.DATA.DATA_PATH, CACHED_PATH, and optionally DO_PREPROCESS.

Usage (from practice-vision/rppg or anywhere):

  python scripts/patch_ubfc_unsupervised_yaml.py \\
    --toolbox-root ~/src/rPPG-Toolbox \\
    --data-path ~/data/UBFC-rPPG \\
    --cache-path ~/data/rppg_toolbox_cache \\
    --set-do-preprocess true

Requires Python 3.8+ (stdlib only).
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def patch_content(text: str, data_path: str, cache_path: str, do_preprocess: bool | None) -> str:
    def esc(s: str) -> str:
        return s.replace("\\", "\\\\").replace('"', '\\"')

    text = re.sub(
        r'^(\s*DATA_PATH:\s*")([^"]*)(")',
        lambda m: f'{m.group(1)}{esc(data_path)}{m.group(3)}',
        text,
        count=1,
        flags=re.MULTILINE,
    )
    text = re.sub(
        r'^(\s*CACHED_PATH:\s*")([^"]*)(")',
        lambda m: f'{m.group(1)}{esc(cache_path)}{m.group(3)}',
        text,
        count=1,
        flags=re.MULTILINE,
    )
    if do_preprocess is not None:
        repl = "True" if do_preprocess else "False"
        text = re.sub(
            r"^(\s*DO_PREPROCESS:\s*)(True|False)",
            rf"\g<1>{repl}",
            text,
            count=1,
            flags=re.MULTILINE,
        )
    return text


def main() -> int:
    p = argparse.ArgumentParser(description="Patch UBFC-rPPG_UNSUPERVISED.yaml paths in rPPG-Toolbox clone.")
    p.add_argument("--toolbox-root", type=Path, required=True, help="Path to rPPG-Toolbox clone")
    p.add_argument("--data-path", type=Path, required=True, help="UBFC-rPPG root (parent of subject1, subject2, …)")
    p.add_argument("--cache-path", type=Path, required=True, help="Directory for preprocessed cache")
    p.add_argument(
        "--set-do-preprocess",
        choices=("true", "false", "omit"),
        default="omit",
        help="If true/false, set UNSUPERVISED.DATA.DO_PREPROCESS (first run: true, later: false)",
    )
    args = p.parse_args()

    toolbox = args.toolbox_root.expanduser().resolve()
    yaml_path = toolbox / "configs" / "infer_configs" / "UBFC-rPPG_UNSUPERVISED.yaml"
    if not yaml_path.is_file():
        print(f"Missing file: {yaml_path}", file=sys.stderr)
        return 1

    data_path = args.data_path.expanduser().resolve()
    cache_path = args.cache_path.expanduser().resolve()
    if not data_path.is_dir():
        print(f"Warning: DATA_PATH is not a directory (yet): {data_path}", file=sys.stderr)

    raw = yaml_path.read_text(encoding="utf-8")
    do_val: bool | None
    if args.set_do_preprocess == "omit":
        do_val = None
    else:
        do_val = args.set_do_preprocess == "true"

    updated = patch_content(raw, str(data_path), str(cache_path), do_val)
    if updated == raw:
        print("No changes applied (patterns not found or same content).", file=sys.stderr)
        return 1

    yaml_path.write_text(updated, encoding="utf-8")
    print(f"Wrote: {yaml_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
