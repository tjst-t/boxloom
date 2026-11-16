#!/bin/sh
# 作るだけで画像にしない(#186 eval F7、f7-notool)。作り手はこれだけを使う。呼ぶたびに runs.log に1行足す。
#
#   sh run.sh pptx tNN      pptx/tNN.py を python3 で流し out/pptx/tNN.pptx を作る
#   sh run.sh boxloom tNN   boxloom/tNN.bx.md を check(JSON)して build し out/boxloom/tNN.pptx を作る
#
# #16 の ../run.sh と違い、PNG に変えない。Boxloom の作り手が見られるのは check の JSON だけ。
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../../.." && pwd)
method=$1; topic=$2
export PATH=/tmp/bxvenv/bin:/usr/local/go/bin:$PATH
out="$HERE/out/$method"; mkdir -p "$out"
pptx="$out/$topic.pptx"
rm -f "$pptx"
status=0
case $method in
pptx)
  (cd "$HERE/pptx" && python3 "$topic.py" "$pptx") || status=$?
  ;;
boxloom)
  bin=/tmp/bxde-f7-notool/boxloom
  if [ ! -x "$bin" ] || [ -n "$(find "$ROOT/boxloom" -name '*.go' -newer "$bin" 2>/dev/null | head -1)" ]; then
    mkdir -p /tmp/bxde-f7-notool && (cd "$ROOT/boxloom" && go build -o "$bin" ./cmd/boxloom) || exit 2
  fi
  echo "== check"
  "$bin" check -format json "$HERE/boxloom/$topic.bx.md"
  echo "== build"
  "$bin" build -o "$pptx" "$HERE/boxloom/$topic.bx.md" || status=$?
  ;;
*) echo "やり方は pptx か boxloom"; exit 2 ;;
esac
printf '%s\t%s\t%s\t%s\n' "$(date +%Y-%m-%dT%H:%M:%S)" "$method" "$topic" "$status" >> "$HERE/runs.log"
if [ -f "$pptx" ]; then
  echo "PPTX: $pptx"
else
  echo "PPTX ができなかった(終了コード $status)"
fi
exit $status
