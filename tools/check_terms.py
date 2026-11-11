#!/usr/bin/env python3
"""リポジトリに書いてはいけない語(業務に関わる固有名など)が入っていないかを検査する。

語の一覧そのものはリポジトリに書かない。環境変数 BOXLOOM_FORBIDDEN_TERMS に
1行1語で渡す(CI では GitHub Actions の秘密から渡す)。

    BOXLOOM_FORBIDDEN_TERMS="$(cat terms.txt)" python3 tools/check_terms.py [--history] [--require]

見るもの:
    - git が追っている全部のファイル(文字のファイル。0 のバイトを含むものは飛ばす)
    - PPTX・DOCX・XLSX の中の XML の文字(zip を開いて見る)
    --history を付けると、全部のコミットの文と文字のファイルの差分(git log -p --all)、
      履歴にある全部の版の PPTX などの中の文字も見る

照らし方:大文字と小文字を区別しない。英数字だけの語は、前後が英数字でないときだけ
当たる(短い略語が別の語の一部に当たらないように)。それ以外の語は部分一致。

画像(PNG など)に描かれた文字は見ない。

終了コード:当たりがあれば 1。一覧が空のときは何もせず 0(--require を付けると 2)。
当たった語はそのまま出さず、一覧の何番目の語かで示す(CI のログに語を残さないため)。
"""
import io
import os
import re
import subprocess
import sys
import zipfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ZIP_EXT = (".pptx", ".docx", ".xlsx")


def load_terms():
    raw = os.environ.get("BOXLOOM_FORBIDDEN_TERMS", "")
    terms = [t.strip() for t in raw.splitlines() if t.strip() and not t.strip().startswith("#")]
    pats = []
    for t in terms:
        if re.fullmatch(r"[A-Za-z0-9]+", t):
            pats.append(re.compile(r"(?<![A-Za-z0-9])" + re.escape(t) + r"(?![A-Za-z0-9])", re.I))
        else:
            pats.append(re.compile(re.escape(t), re.I))
    return pats


def scan_text(text, pats):
    hits = []
    for i, p in enumerate(pats):
        for m in p.finditer(text):
            line = text.count("\n", 0, m.start()) + 1
            hits.append((i + 1, line))
    return hits


def git(*args):
    return subprocess.run(["git", *args], cwd=ROOT, capture_output=True, check=True).stdout


def scan_files(pats):
    found = []
    for name in git("ls-files", "-z").decode().split("\0"):
        if not name:
            continue
        path = os.path.join(ROOT, name)
        try:
            data = open(path, "rb").read()
        except OSError:
            continue
        if name.lower().endswith(ZIP_EXT):
            scan_zip(name, data, pats, found)
            continue
        if b"\0" in data:
            continue
        for term, line in scan_text(data.decode("utf-8", "replace"), pats):
            found.append(f"{name}:{line}: term #{term}")
    return found


def scan_zip(label, data, pats, found):
    try:
        z = zipfile.ZipFile(io.BytesIO(data))
    except zipfile.BadZipFile:
        return
    for part in z.namelist():
        if part.endswith((".xml", ".rels")):
            for term, line in scan_text(z.read(part).decode("utf-8", "replace"), pats):
                found.append(f"{label}!/{part}:{line}: term #{term}")


def scan_history(pats):
    """全部のコミットの文と、文字のファイルの差分。PPTX などは履歴にある全部の版を開いて見る。
    PNG などの中身は見ない(圧縮したバイト列が偶然語の並びになることがあるため)。"""
    found = []
    log = git("log", "--all", "-p", "--format=commit %H%n%B", "--no-color").decode("utf-8", "replace")
    commit = "?"
    for line in log.splitlines():
        if line.startswith("commit ") and len(line) == 47:
            commit = line[7:19]
        for i, p in enumerate(pats):
            if p.search(line):
                found.append(f"history {commit}: term #{i + 1}")
    seen = set()
    for row in git("rev-list", "--all", "--objects").decode().splitlines():
        sha, _, path = row.partition(" ")
        if path.lower().endswith(ZIP_EXT) and sha not in seen:
            seen.add(sha)
            scan_zip(f"history {path}@{sha[:12]}", git("cat-file", "blob", sha), pats, found)
    return sorted(set(found))


def main(argv):
    pats = load_terms()
    if not pats:
        msg = "BOXLOOM_FORBIDDEN_TERMS is empty; nothing checked"
        if "--require" in argv:
            print(msg, file=sys.stderr)
            return 2
        print(msg)
        return 0
    found = scan_files(pats)
    if "--history" in argv:
        found += scan_history(pats)
    for f in found:
        print(f)
    print(f"{len(pats)} term(s), {len(found)} hit(s)")
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
