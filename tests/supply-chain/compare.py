#!/usr/bin/env python3
"""Compare a declared DeskOS SBOM against an installed (scanner) SBOM.

The declared SBOM (`deskosctl sbom`) tags each component with a
``deskos:source`` property. Only ``rpm`` components are gated here: a
declared RPM package must appear, by name, in the installed SBOM. The
other kinds (RPM files, binaries, Flatpaks, trust anchors) are reported
but not gated, because a scanner does not enumerate all of them and the
boot/session checks and the generated manifest cover those.

Exit 0 when no declared RPM package is missing, 1 otherwise, 2 on usage.

Usage: compare.py DECLARED.cdx.json INSTALLED.cdx.json
"""
from __future__ import annotations

import json
import sys


def load_components(path):
    with open(path, encoding="utf-8") as fh:
        doc = json.load(fh)
    if doc.get("bomFormat") != "CycloneDX":
        raise ValueError(f"{path}: not a CycloneDX document")
    return doc.get("components") or []


def source(component):
    for prop in component.get("properties") or []:
        if prop.get("name") == "deskos:source":
            return prop.get("value")
    return None


def compare(declared, installed):
    """Return (missing_rpm_names, count_of_other_declared_components)."""
    installed_names = {(c.get("name") or "").lower() for c in installed}
    missing = []
    others = 0
    for c in declared:
        if source(c) == "rpm":
            name = c.get("name") or ""
            if name.lower() not in installed_names:
                missing.append(name)
        else:
            others += 1
    return missing, others


def main(argv):
    if len(argv) != 3:
        print("usage: compare.py DECLARED.cdx.json INSTALLED.cdx.json", file=sys.stderr)
        return 2
    declared = load_components(argv[1])
    installed = load_components(argv[2])
    missing, others = compare(declared, installed)
    declared_rpm = sum(1 for c in declared if source(c) == "rpm")
    print(f"declared RPM packages: {declared_rpm}")
    print(f"missing from the installed SBOM: {len(missing)}")
    for name in missing:
        print(f"  missing: {name}")
    print(f"declared components not gated here (files, binaries, Flatpaks, anchors): {others}")
    return 1 if missing else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
