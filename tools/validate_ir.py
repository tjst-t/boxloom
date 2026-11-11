#!/usr/bin/env python3
"""Boxloom の IR(意味の中間表現)を JSON Schema で検査する。

使い方:
    python3 tools/validate_ir.py FILE.json ...   # 検査する。誤りは JSON で標準出力に出す
    python3 tools/validate_ir.py --self-test     # 例の正しいものが通り、断られるべきものが断られるか

Schema で書けない検査(id の一意性・target の指す先・日付の前後など)は
Go の `go run . check deck.json`(boxloom/check.go)がする。

要るもの: pip install jsonschema (4.18 以降。referencing を使う)

誤りは docs/content-syntax.md の5章の形で返す(Go の boxloom check・Markdown の
パーサーと同じ):

    {"file": "deck.json", "path": "/slides/2/...", "code": "schema",
     "message": "...", "expected": [...], "hint": "..."}

IR(JSON)を検査するので line・column は無い。code はいつも "schema" で、
どの Schema の規則に当たったか(enum・additionalProperties など)は hint に書く。
expected は、選べる値が決まっているとき(enum・const)だけ付ける。
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
    if not path:
        return ""
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
        issue = {
            "file": str(path),
            "path": pointer(leaf.absolute_path),
            "code": "schema",
            "message": leaf.message,
        }
        if leaf.validator == "enum":
            issue["expected"] = [str(v) for v in leaf.validator_value]
        elif leaf.validator == "const":
            issue["expected"] = [str(leaf.validator_value)]
        issue["hint"] = f"Schema の規則: {leaf.validator}"
        errors.append(issue)
    return errors


def self_test(validator):
    ok = True
    for path in sorted((ROOT / "examples" / "ir").glob("*.json")):
        errs = check(validator, path)
        print(f"{'PASS' if not errs else 'FAIL'}  valid    {path.relative_to(ROOT)}")
        for e in errs:
            print("      ", json.dumps(e, ensure_ascii=False))
        ok &= not errs
    # Schema では通り、意味の検査(boxloom check)で断るもの。Schema が誤って
    # 断るようになっていないかを見る(断るのは Go の試験 check_test.go)。
    for path in sorted((ROOT / "examples" / "ir" / "check-invalid").glob("*.json")):
        errs = check(validator, path)
        print(f"{'PASS' if not errs else 'FAIL'}  schema-ok {path.relative_to(ROOT)}  (boxloom check rejects)")
        for e in errs:
            print("      ", json.dumps(e, ensure_ascii=False))
        ok &= not errs
    for path in sorted((ROOT / "examples" / "ir" / "invalid").glob("*.json")):
        errs = check(validator, path)
        print(f"{'PASS' if errs else 'FAIL'}  rejected {path.relative_to(ROOT)}"
              + (f"  -> {errs[0]['path']}: {errs[0]['message']}" if errs else ""))
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
