#!/usr/bin/env python3
"""Replace original executable captures with SHA-256 regression expectations."""

import argparse
import base64
import hashlib
import json
from pathlib import Path


def sanitize(value, source):
    """Keep numeric state and replace captured requester/code byte regions."""
    changed = False
    if isinstance(value, dict):
        for field, payload in list(value.items()):
            capture = field == "Scratch" or (
                field == "Window"
                and source.name in {"file_frame_native.json", "dos_overwrite_native.json"}
            )
            if capture and isinstance(payload, str):
                raw = base64.b64decode(payload, validate=True)
                value[field + "Hash"] = hashlib.sha256(raw).hexdigest()
                del value[field]
                changed = True
            else:
                changed = sanitize(payload, source) or changed
    elif isinstance(value, list):
        for item in value:
            changed = sanitize(item, source) or changed
    return changed


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("paths", type=Path, nargs="*", help="fixture files to sanitize")
    args = parser.parse_args()
    default = Path(__file__).resolve().parents[1] / "internal/populous2/testdata"
    paths = args.paths or sorted(default.glob("*.json"))
    for path in paths:
        data = json.loads(path.read_text())
        if sanitize(data, path):
            path.write_text(json.dumps(data, separators=(",", ":")) + "\n")
            print(path)


if __name__ == "__main__":
    main()
