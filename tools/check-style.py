#!/usr/bin/env python3
"""Check the mechanical part of docs/style.md.

Flags em dashes, en dashes in prose lines and a list of filler words, in prose
only: fenced code, inline code and link targets are ignored.

Usage: python3 tools/check-style.py [--list]
Exits non-zero when anything is found.
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SKIP_DIRS = {".git", ".deskos-supervisor", "bin", "node_modules", "testdata"}
# style.md lists the banned words as examples, so it would always fail.
SKIP_FILES = {"docs/style.md"}
FILLER = [
    "it is worth noting", "in practice", "essentially", "at its core",
    "robust", "seamless", "comprehensive", "leverage", "delves", "delving",
    "moreover", "furthermore", "navigate the landscape", "not just",
]
INLINE_CODE = re.compile(r"`[^`]*`")
LINK_TARGET = re.compile(r"\]\([^)]*\)")


def prose(text):
    """Lines outside fenced code, without inline code or link targets."""
    fenced = False
    for number, line in enumerate(text.splitlines(), 1):
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if fenced:
            continue
        yield number, INLINE_CODE.sub("", LINK_TARGET.sub("]", line))


def files():
    for base, dirs, names in os.walk(ROOT):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for name in names:
            if name.endswith(".md"):
                yield os.path.join(base, name)


def main():
    hits = []
    for path in files():
        rel = os.path.relpath(path, ROOT)
        if rel in SKIP_FILES:
            continue
        with open(path, encoding="utf-8", errors="replace") as fh:
            text = fh.read()
        for number, line in prose(text):
            if "\u2014" in line:
                hits.append(f"{rel}:{number}: em dash")
            if re.search(r"\d\u2013\d", line) is None and "\u2013" in line:
                hits.append(f"{rel}:{number}: en dash in prose")
            low = line.lower()
            for word in FILLER:
                if word in low:
                    hits.append(f"{rel}:{number}: filler: {word}")
    for hit in hits:
        print(hit)
    print(f"{len(hits)} style finding(s) in {len(list(files()))} files")
    return 1 if hits else 0


if __name__ == "__main__":
    sys.exit(main())
