#!/usr/bin/env python3
"""Boxloom の IR(意味の中間表現)を JSON Schema で検査する。

使い方:
    python3 tools/validate_ir.py FILE.json ...   # 検査する。誤りは JSON で標準出力に出す
    python3 tools/validate_ir.py --self-test     # 例の正しいものが通り、断られるべきものが断られるか

要るもの: pip install jsonschema (4.18 以降。referencing を使う)

誤りは「ファイル・IR の中の場所(JSON Pointer)・何が問題か」の形で返す。
AI に読ませて直させることを考えて、機械が読める形にしてある。
"""
import json
import pathlib
import sys

from jsonschema import Draft202012Validator
from referencing import Registry, Resource

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMA_DIR = ROOT / "schema"
ENTRY = "https://boxloom.invalid/schema/ir/0.1/boxloom-ir.schema.json"


def load_registry():
    resources = []
    for path in sorted(SCHEMA_DIR.rglob("*.schema.json")):
        schema = json.loads(path.read_text(encoding="utf-8"))
        resources.append((schema["$id"], Resource.from_contents(schema)))
    return Registry().with_resources(resources)


def make_validator():
    registry = load_registry()
    entry = registry.contents(ENTRY)
    Draft202012Validator.check_schema(entry)
    return Draft202012Validator(entry, registry=registry)


def pointer(path):
    return "/" + "/".join(str(p).replace("~", "~0").replace("/", "~1") for p in path)


def deepest(error):
    """oneOf の中の誤りは、一番深いところまで降りたものを出す(どこを直すかが分かるように)。"""
    best = error
    for sub in error.context or []:
        cand = deepest(sub)
        if len(cand.absolute_path) > len(best.absolute_path):
            best = cand
    return best


def check(validator, path):
    doc = json.loads(pathlib.Path(path).read_text(encoding="utf-8"))
    errors = []
    for err in sorted(validator.iter_errors(doc), key=lambda e: list(map(str, e.absolute_path))):
        leaf = deepest(err)
        errors.append({
            "file": str(path),
            "at": pointer(leaf.absolute_path),
            "message": leaf.message,
            "rule": leaf.validator,
        })
    return errors


def self_test(validator):
    ok = True
    for path in sorted((ROOT / "examples" / "ir").glob("*.json")):
        errs = check(validator, path)
        print(f"{'PASS' if not errs else 'FAIL'}  valid    {path.relative_to(ROOT)}")
        for e in errs:
            print("      ", json.dumps(e, ensure_ascii=False))
        ok &= not errs
    for path in sorted((ROOT / "examples" / "ir" / "invalid").glob("*.json")):
        errs = check(validator, path)
        print(f"{'PASS' if errs else 'FAIL'}  rejected {path.relative_to(ROOT)}"
              + (f"  -> {errs[0]['at']}: {errs[0]['message']}" if errs else ""))
        ok &= bool(errs)
    return ok


def main(argv):
    validator = make_validator()
    if argv == ["--self-test"]:
        return 0 if self_test(validator) else 1
    if not argv:
        print(__doc__)
        return 2
    all_errors = [e for p in argv for e in check(validator, p)]
    print(json.dumps({"ok": not all_errors, "errors": all_errors}, ensure_ascii=False, indent=2))
    return 0 if not all_errors else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
