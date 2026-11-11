#!/usr/bin/env python3
"""ECMA-376 の XML Schema(XSD)を取ってきて tools/.ooxml-schemas/ に置く。

tools/validate_pptx.py が PPTX の各部品をこの Schema で検査する。Schema は
リポジトリに入れない(大きく、ECMA の配布物なので)。取ってくる zip は
sha256 で中身を確かめる。

    python3 tools/fetch_ooxml_schemas.py [--dest DIR]

置くもの:
    DIR/transitional/*.xsd   ECMA-376 Part 4(5th edition, 2016)の Transitional の XSD。
                             PowerPoint が書く名前空間(schemas.openxmlformats.org)はこちら
    DIR/opc/*.xsd            ECMA-376 Part 2(5th edition, 2021)の Open Packaging Conventions

標準ライブラリだけで動く。
"""
import argparse
import hashlib
import io
import pathlib
import sys
import urllib.request
import zipfile

ROOT = pathlib.Path(__file__).resolve().parent
DEFAULT_DEST = ROOT / ".ooxml-schemas"

SOURCES = [
    {
        "url": "https://ecma-international.org/wp-content/uploads/ECMA-376-4_5th_edition_december_2016.zip",
        "sha256": "bd25da1109f73762356596918bf5ff8b74a1331642dba5f1c1d1dfc6bed34ecd",
        "inner": "OfficeOpenXML-XMLSchema-Transitional.zip",
        "inner_sha256": "d34187520749998af306faf1b730e568b0ca6d88ad24638a407c0a9bb4ca04fc",
        "dir": "transitional",
    },
    {
        "url": "https://ecma-international.org/wp-content/uploads/ECMA-376-2_5th_edition_december_2021.zip",
        "sha256": "1d489dc491168ea1f9e9a59063acc8dd5f02b4ad1d21aa7ec19ba9a58d020c70",
        "inner": "OpenPackagingConventions-XMLSchema.zip",
        "inner_sha256": "e5a0a7c1f43ac8b8f66d850f48545c2ead09f6d632bb8539fab79962c7fd55df",
        "dir": "opc",
    },
]


def sha256(b):
    return hashlib.sha256(b).hexdigest()


def fetch(src, dest):
    out = dest / src["dir"]
    if out.is_dir() and any(out.glob("*.xsd")):
        print(f"already present: {out}")
        return
    print(f"downloading {src['url']}")
    with urllib.request.urlopen(src["url"], timeout=300) as r:
        data = r.read()
    if sha256(data) != src["sha256"]:
        sys.exit(f"sha256 mismatch for {src['url']}")
    inner = zipfile.ZipFile(io.BytesIO(data)).read(src["inner"])
    if sha256(inner) != src["inner_sha256"]:
        sys.exit(f"sha256 mismatch for {src['inner']}")
    out.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(io.BytesIO(inner)) as z:
        for name in z.namelist():
            if name.endswith(".xsd"):
                (out / pathlib.Path(name).name).write_bytes(z.read(name))
    print(f"wrote {out}")


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--dest", default=str(DEFAULT_DEST))
    a = ap.parse_args()
    dest = pathlib.Path(a.dest)
    for s in SOURCES:
        fetch(s, dest)


if __name__ == "__main__":
    main()
