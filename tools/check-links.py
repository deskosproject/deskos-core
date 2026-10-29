#!/usr/bin/env python3
"""Check relative Markdown links and anchors across the repository.

Usage: python3 tools/check-links.py
Exits non-zero and lists every link whose target file or anchor is missing.
Fenced code blocks are skipped.
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LINK = re.compile(r"\[[^\]]*\]\(([^)]+)\)")


def md_files():
    for base, dirs, files in os.walk(ROOT):
        dirs[:] = [d for d in dirs if d not in (".git", ".deskos-supervisor", "bin", "node_modules")]
        for name in files:
            if name.endswith(".md"):
                yield os.path.join(base, name)


def slug(heading):
    text = heading.strip().lower()
    text = re.sub(r"[^\w\s-]", "", text)
    return re.sub(r"\s+", "-", text)


def anchors(path):
    out = set()
    fenced = False
    with open(path, encoding="utf-8", errors="replace") as fh:
        for line in fh:
            if line.lstrip().startswith("```"):
                fenced = not fenced
                continue
            if fenced:
                continue
            m = re.match(r"#{1,6}\s+(.*)", line)
            if m:
                out.add(slug(m.group(1)))
    return out


def main():
    broken = []
    for path in md_files():
        rel = os.path.relpath(path, ROOT)
        fenced = False
        with open(path, encoding="utf-8", errors="replace") as fh:
            for number, line in enumerate(fh, 1):
                if line.lstrip().startswith("```"):
                    fenced = not fenced
                    continue
                if fenced:
                    continue
                for target in LINK.findall(line):
                    target = target.split(" ")[0]
                    if target.startswith(("http://", "https://", "mailto:", "#")):
                        continue
                    file_part, _, anchor = target.partition("#")
                    if not file_part:
                        dest = path
                    else:
                        dest = os.path.normpath(os.path.join(os.path.dirname(path), file_part))
                    if not os.path.exists(dest):
                        broken.append(f"{rel}:{number}: missing file {target}")
                        continue
                    if anchor and dest.endswith(".md") and anchor not in anchors(dest):
                        broken.append(f"{rel}:{number}: missing anchor {target}")
    for item in broken:
        print(item)
    print(f"{len(broken)} broken link(s)")
    return 1 if broken else 0


if __name__ == "__main__":
    sys.exit(main())
