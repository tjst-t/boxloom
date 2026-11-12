#!/bin/sh
# CI の手順。GitHub Actions(.github/workflows/ci.yml)もローカルもこれを流す。
#
#   sh tools/ci.sh [手順 ...]      手順を省くと go python pptx terms を順に
#
# 手順:
#   go        gofmt の差・go vet・go test
#   python    validate_ir.py --self-test(schema/ で)と --bundle(boxloom schema の出力で)、
#             validate_layout.py --self-test(boxloom layout の出力を schema/layout.schema.json で)
#   pptx      validate_pptx.py --self-test(XSD は無ければ取ってくる)
#   readers   validate_pptx.py --self-test --soffice(LibreOffice が要る。重いので既定では流さない)
#   terms     check_terms.py --history(一覧は環境変数 BOXLOOM_FORBIDDEN_TERMS。
#             CI では BOXLOOM_REQUIRE_TERMS=1 にして、一覧が空なら落とす)
#
# 要るもの:Go 1.24、Python 3 と pip の jsonschema・lxml・python-pptx。
# 入れ方は docs/build.md の「CI と検査の道具」。
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

step_go() {
  echo "== go"
  unformatted=$(gofmt -l boxloom)
  if [ -n "$unformatted" ]; then
    echo "gofmt が要るファイル:"; echo "$unformatted"; exit 1
  fi
  (cd boxloom && go vet ./... && go test -count=1 ./...)
}

step_python() {
  echo "== python"
  python3 tools/validate_ir.py --self-test
  bundle=$(mktemp)
  (cd boxloom && go run ./cmd/boxloom schema) > "$bundle"
  python3 tools/validate_ir.py --bundle "$bundle" --self-test
  rm -f "$bundle"
  python3 tools/validate_layout.py --self-test
}

step_pptx() {
  echo "== pptx"
  python3 tools/fetch_ooxml_schemas.py
  python3 tools/validate_pptx.py --self-test
}

step_readers() {
  echo "== readers"
  python3 tools/fetch_ooxml_schemas.py
  python3 tools/validate_pptx.py --self-test --soffice
}

step_terms() {
  echo "== terms"
  if [ "${BOXLOOM_REQUIRE_TERMS:-}" = "1" ]; then
    python3 tools/check_terms.py --history --require
  else
    python3 tools/check_terms.py --history
  fi
}

[ $# -eq 0 ] && set -- go python pptx terms
for s in "$@"; do
  case "$s" in
    go|python|pptx|readers|terms) "step_$s" ;;
    *) echo "unknown step: $s" >&2; exit 2 ;;
  esac
done
echo "== ok: $*"
