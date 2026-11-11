#!/usr/bin/env python3
"""字体1つ分の寸法(文字の幅)を測って boxloom/fonts/<slug>.json に書く。

Go の本体は字体のファイルを読まない(依存ゼロ)。代わりに、ここで一度だけ
Pillow で測った「1em あたりの送り幅」を JSON にし、バイナリに同梱する。
字体のファイルそのものはリポジトリに入れない(ライセンスのため)。寸法は
数の表だけ。

使い方:
    python3 tools/export_font_metrics.py \
        --family "Noto Sans CJK JP" \
        --regular /usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc \
        --bold    /usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc \
        --license OFL-1.1 \
        --out boxloom/fonts/noto-sans-cjk-jp.json

    太字のファイルが無い字体(IPAex など)は --bold を省く。そのときは
    太字にも標準の幅を使う(PowerPoint の擬似太字は送り幅を変えない前提。
    docs/fonts.md)。

測る文字:ASCII・ラテン1補助・ラテン拡張A・一般句読点・矢印・囲み英数字・
幾何学模様・その他の記号・CJK の記号と句読点・ひらがな・カタカナ・
全角形。japanese は仮名と漢字の字形を持つか(和文の字体に選べるか)。漢字は 1 文字(「一」)を測って default にする(和文の字体は
漢字がすべて同じ幅)。表に載せるのは default と違う幅の文字だけ。
字体に無い文字(.notdef になる文字)は載せない(Go 側は default を使う)。

要るもの:Pillow(python3 -c "import PIL")。
"""

import argparse
import json
import sys

from PIL import ImageFont

REF = 1000  # この大きさ(px)で測って REF で割ると em の比になる

RANGES = [
    (0x20, 0x7E),      # ASCII
    (0xA0, 0xFF),      # ラテン1補助
    (0x100, 0x17F),    # ラテン拡張A
    (0x2000, 0x206F),  # 一般句読点(– — ‘ ’ “ ” … ※ など)
    (0x2190, 0x21FF),  # 矢印
    (0x2460, 0x24FF),  # 囲み英数字(① など)
    (0x25A0, 0x25FF),  # 幾何学模様(■ ○ ▲ など)
    (0x2600, 0x26FF),  # その他の記号(★ など)
    (0x3000, 0x303F),  # CJK の記号と句読点(、。「」 など)
    (0x3040, 0x309F),  # ひらがな
    (0x30A0, 0x30FF),  # カタカナ
    (0xFF00, 0xFFEF),  # 全角形・半角形
]


def has_glyph(font, ch, missing_mask):
    """ch が .notdef(豆腐)以外の字形を持つか。字形の無い文字と同じ絵なら無いとみなす。"""
    try:
        m = font.getmask(ch)
    except Exception:
        return False
    return not (m.size == missing_mask.size and bytes(m) == bytes(missing_mask))


def measure(path):
    font = ImageFont.truetype(path, size=REF)
    missing = font.getmask("\U0010FFFD")
    widths = {}
    for lo, hi in RANGES:
        for cp in range(lo, hi + 1):
            ch = chr(cp)
            # 空白は絵が無いので字形の有無を見ない
            if cp not in (0x20, 0xA0, 0x3000) and not (0x2000 <= cp <= 0x200A) and not has_glyph(font, ch, missing):
                continue
            widths[ch] = round(font.getlength(ch) / REF, 6)
    default = round(font.getlength("一") / REF, 6)
    asc, desc = font.getmetrics()
    japanese = all(has_glyph(font, ch, missing) for ch in "あア一")
    return widths, default, asc / REF, desc / REF, japanese


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--family", required=True, help="PPTX に書く字体の名前(typeface)")
    ap.add_argument("--regular", required=True, help="標準の太さの字体のファイル")
    ap.add_argument("--bold", help="太字の字体のファイル(無ければ標準の幅を使う)")
    ap.add_argument("--license", required=True, help="字体のライセンス(例 OFL-1.1)")
    ap.add_argument("--note", default="", help="出どころなどのメモ")
    ap.add_argument("--out", required=True)
    a = ap.parse_args()

    reg, default, asc, desc, japanese = measure(a.regular)
    if a.bold:
        bold, bold_default, _, _, _ = measure(a.bold)
    else:
        bold, bold_default = reg, default

    def sparse(table, dflt):
        return {ch: w for ch, w in sorted(table.items(), key=lambda kv: ord(kv[0])) if w != dflt}

    note = a.note or ("measured with Pillow from %s%s" % (
        a.regular.rsplit("/", 1)[-1],
        (" and " + a.bold.rsplit("/", 1)[-1]) if a.bold else " (bold uses the regular widths)"))
    out = {
        "family": a.family,
        "source": "measured",
        "license": a.license,
        "japanese": japanese,
        "note": note,
        "ascent": round(asc, 4),
        "descent": round(desc, 4),
        "regular": {"default": default, "widths": sparse(reg, default)},
        "bold": {"default": bold_default, "widths": sparse(bold, bold_default)},
    }
    with open(a.out, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=1)
        f.write("\n")
    print("wrote %s: %d regular / %d bold widths (default %.3f em), ascent+descent %.3f em" % (
        a.out, len(out["regular"]["widths"]), len(out["bold"]["widths"]), default, asc + desc), file=sys.stderr)


if __name__ == "__main__":
    main()
