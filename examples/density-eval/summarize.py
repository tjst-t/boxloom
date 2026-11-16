#!/usr/bin/env python3
"""out/<やり方>/tNN.pptx を tools/density_metrics.py で測り、runs.log の回数と合わせて表にする。

  python3 summarize.py            results.json を書き、Markdown の表を出す(docs/eval.md の3節)
  python3 summarize.py noimage    noimage/ の下(画像なしの作り手、#182)を同じく測る(docs/eval.md の7節)
  python3 summarize.py f7-haiku   小さいモデル(Haiku 5.5)の作り手、画像なし(#186、docs/eval.md の8節)
  python3 summarize.py f7-notool  測る道具を書くことを禁じた Opus 5.5、画像なし(#186、8節)

本文の3枚(表紙の1枚目を除く)を測る。作り直しの回数は run.sh を流した回数 − 1。
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
TOPICS = ["t%02d" % i for i in range(1, 11)]
METHODS = ["pptx", "boxloom"]


def runs(base):
    n = {}
    with open(os.path.join(base, "runs.log"), encoding="utf-8") as f:
        for line in f:
            parts = line.rstrip("\n").split("\t")
            if len(parts) >= 3:
                n[(parts[1], parts[2])] = n.get((parts[1], parts[2]), 0) + 1
    return n


def main():
    base = os.path.join(HERE, sys.argv[1]) if len(sys.argv) > 1 else HERE
    counts = runs(base)
    files = [os.path.join(base, "out", m, t + ".pptx") for m in METHODS for t in TOPICS]
    missing = [f for f in files if not os.path.exists(f)]
    if missing:
        print("無い PPTX:", missing, file=sys.stderr)
        return 1
    out = subprocess.run([sys.executable, os.path.join(ROOT, "tools", "density_metrics.py"), "-skip-first"] + files,
                         check=True, capture_output=True, text=True).stdout
    measured = {os.path.relpath(d["file"], os.path.join(base, "out")): d for d in json.loads(out)}
    results = []
    for m in METHODS:
        for t in TOPICS:
            d = measured[os.path.join(m, t + ".pptx")]
            sl = d["slides"]
            k = len(sl)
            results.append({
                "method": m, "topic": t, "slides": k,
                "elements": round(sum(s["elements"] for s in sl) / k, 1),
                "chars": round(sum(s["chars"] for s in sl) / k),
                "overlaps": sum(s["overlaps"] for s in sl),
                "overflows": sum(s["overflows"] for s in sl),
                "shrunk": sum(s["shrunk"] for s in sl),
                "fill": round(sum(s["fill"] for s in sl) / k, 3),
                "minFont": min((s["minFont"] for s in sl if s["minFont"]), default=None),
                "meanFont": round(sum(s["meanFont"] * s["chars"] for s in sl if s["meanFont"]) /
                                  max(1, sum(s["chars"] for s in sl if s["meanFont"])), 1),
                "rebuilds": counts.get((m, t), 0) - 1,
                "detail": sl,
            })
    with open(os.path.join(base, "results.json"), "w", encoding="utf-8") as f:
        json.dump(results, f, ensure_ascii=False, indent=1)
        f.write("\n")
    head = "| 題材 | やり方 | 本文の枚 | 要素/枚 | 文字/枚 | 重なり | はみ出し | 埋まり | 字の最小/平均(pt) | 作り直し |"
    print(head)
    print("|---" * 10 + "|")
    for r in sorted(results, key=lambda r: (r["topic"], r["method"])):
        print("| %s | %s | %d | %.1f | %d | %d | %d | %.0f%% | %s / %s | %d |" % (
            r["topic"], r["method"], r["slides"], r["elements"], r["chars"], r["overlaps"], r["overflows"],
            r["fill"] * 100, r["minFont"], r["meanFont"], r["rebuilds"]))
    print()
    for m in METHODS:
        rs = [r for r in results if r["method"] == m]
        n = len(rs)
        print("| 合計・平均 | %s | %d | %.1f | %d | %d | %d | %.0f%% | %s / %.1f | %.1f |" % (
            m, sum(r["slides"] for r in rs), sum(r["elements"] for r in rs) / n, sum(r["chars"] for r in rs) / n,
            sum(r["overlaps"] for r in rs), sum(r["overflows"] for r in rs), sum(r["fill"] for r in rs) / n * 100,
            min(r["minFont"] for r in rs), sum(r["meanFont"] for r in rs) / n, sum(r["rebuilds"] for r in rs) / n))
    return 0


if __name__ == "__main__":
    sys.exit(main())
