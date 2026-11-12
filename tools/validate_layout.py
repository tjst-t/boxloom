#!/usr/bin/env python3
"""`boxloom layout` の出力(配置の段の木)を schema/layout.schema.json で検査する。

使い方:
    python3 tools/validate_layout.py FILE.json ...   # layout の出力を検査する
    python3 tools/validate_layout.py --self-test     # examples の全 deck を全テンプレートで
                                                     # layout に通し、出力が Schema に通るか

--self-test は boxloom/ で `go build ./cmd/boxloom` をした実行ファイルを一時の
フォルダに作って使う(Go が要る)。box が PPTX の図形と一致することは Go の試験
(layoutdump_test.go)が確かめる。

要るもの: pip install jsonschema
"""
import json
import pathlib
import subprocess
import sys
import tempfile

from jsonschema import Draft202012Validator

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMA = ROOT / "schema" / "layout.schema.json"
TEMPLATES = ["", "dark", "light", str(ROOT / "examples" / "templates" / "forest.json")]


def make_validator():
    schema = json.loads(SCHEMA.read_text(encoding="utf-8"))
    Draft202012Validator.check_schema(schema)
    return Draft202012Validator(schema)


def errors_of(validator, doc):
    return [f"/{'/'.join(map(str, e.absolute_path))}: {e.message}"
            for e in sorted(validator.iter_errors(doc), key=lambda e: list(map(str, e.absolute_path)))]


def decks():
    ex = ROOT / "examples"
    return (sorted((ex / "ir").glob("*.json")) + sorted((ex / "ir" / "check-warnings").glob("*.json"))
            + sorted((ex / "md").glob("*.bx.md")) + sorted((ex / "md" / "warnings").glob("*.bx.md"))
            + sorted((ex / "yaml").glob("*.yaml")))


def self_test(validator):
    ok = True
    with tempfile.TemporaryDirectory() as tmp:
        exe = pathlib.Path(tmp) / "boxloom"
        subprocess.run(["go", "build", "-o", str(exe), "./cmd/boxloom"], cwd=ROOT / "boxloom", check=True)
        for deck in decks():
            for tmpl in TEMPLATES:
                args = [str(exe), "layout"] + (["-template", tmpl] if tmpl else []) + [str(deck)]
                p = subprocess.run(args, capture_output=True, text=True)
                label = f"{deck.relative_to(ROOT)} @{pathlib.Path(tmpl).stem if tmpl else ''}"
                if p.returncode != 0:
                    print(f"FAIL  {label}: exit {p.returncode}: {p.stderr.strip()}")
                    ok = False
                    continue
                errs = errors_of(validator, json.loads(p.stdout))
                print(f"{'PASS' if not errs else 'FAIL'}  layout {label}")
                for e in errs[:5]:
                    print("      ", e)
                ok &= not errs
    return ok


def main(argv):
    validator = make_validator()
    if argv == ["--self-test"]:
        return 0 if self_test(validator) else 1
    if not argv:
        print(__doc__)
        return 2
    bad = False
    for path in argv:
        errs = errors_of(validator, json.loads(pathlib.Path(path).read_text(encoding="utf-8")))
        for e in errs:
            print(f"{path}: {e}")
        bad |= bool(errs)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
