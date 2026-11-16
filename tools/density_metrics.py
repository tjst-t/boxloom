#!/usr/bin/env python3
"""PPTX を同じ物差しで測る(#16 density-eval、docs/eval.md)。

作り方(python-pptx の直書きか Boxloom か)によらず、できた PPTX だけを見て
1枚ごとに次を数える。

  elements   見える葉の要素の数(グループは中の葉を数える。表・グラフは1つ)
  chars      文字の数(空白を除く。図形・表のセル・グラフの項目と系列の名前)
  overlaps   重なりの組の数。線・コネクタと、スライドの大半を覆う地は除く。
             (a) どちらも相手を含まない交わり(部分の重なり)か、
             (b) 文字を持つ要素どうしの交わり(含まれていても数える)
  overflows  はみ出しの数。(a) 文字が枠に入りきらない図形(行の数を字体の寸法で
             見積もる)と、(b) スライドの外に出る要素(表は行が伸びた後の高さで見る)
  fill       埋まり具合。要素の矩形の和の面積 ÷ スライドの面積(地は除く)
  minFont / meanFont  字の大きさの最小と、文字数で重みをつけた平均(pt)

文字の幅は boxloom/fonts/noto-sans-cjk-jp.json の寸法で測る(Boxloom と同じ寸法)。
行の送りは lnSpc の spcPts があればその値、spcPct なら 字の大きさ×1.2×倍率
(LibreOffice で測った単一の行の送り)。自動の縮小(normAutofit)の枠ははみ出しに
数えず、shrunk に数える。

  python3 tools/density_metrics.py [-skip-first] a.pptx b.pptx ...   JSON を出す
"""
import json
import os
import re
import sys

from lxml import etree
from pptx import Presentation
from pptx.util import Emu

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
NS = {
    "a": "http://schemas.openxmlformats.org/drawingml/2006/main",
    "p": "http://schemas.openxmlformats.org/presentationml/2006/main",
    "c": "http://schemas.openxmlformats.org/drawingml/2006/chart",
}
EMU_PT = 12700.0
SINGLE = 1.2  # spcPct 100% の行の送り(字の大きさに掛ける)
TOL = 2.0  # pt。はみ出し・重なりの許し

with open(os.path.join(ROOT, "boxloom", "fonts", "noto-sans-cjk-jp.json"), encoding="utf-8") as f:
    _FONT = json.load(f)


def char_width(ch, bold):
    face = _FONT["bold" if bold else "regular"]
    w = face["widths"].get(ch)
    if w is not None:
        return w
    if ord(ch) < 0x2E80:
        return 0.6 if not ch.isspace() else 0.25
    return face["default"]


TOKEN = re.compile(r"[A-Za-z0-9.,%:/+\-]+|\s|.", re.S)


def wrap_lines(runs, width):
    """runs: [(text, size, bold)]。1つの段落を幅 width(pt)で折り返した行ごとの最大の字の大きさ。"""
    if width <= 0:
        width = 1
    lines = []
    cur_w = 0.0
    cur_size = 0.0
    any_text = False
    for text, size, bold in runs:
        for tok in TOKEN.findall(text):
            if tok == "\n" or tok == "\v":
                lines.append(cur_size or size)
                cur_w, cur_size = 0.0, 0.0
                continue
            w = sum(char_width(c, bold) for c in tok) * size
            if cur_w + w > width + 0.01 and cur_w > 0 and not tok.isspace():
                lines.append(cur_size or size)
                cur_w, cur_size = 0.0, 0.0
            if w > width and not tok.isspace():  # 1語が幅より長い:字ごとに折る
                for c in tok:
                    cw = char_width(c, bold) * size
                    if cur_w + cw > width + 0.01 and cur_w > 0:
                        lines.append(cur_size or size)
                        cur_w, cur_size = 0.0, 0.0
                    cur_w += cw
                    cur_size = max(cur_size, size)
            else:
                cur_w += w
                cur_size = max(cur_size, size)
            any_text = True
    lines.append(cur_size or (runs[-1][1] if runs else 18.0))
    return lines, any_text


def widest_line(runs):
    best, cur = 0.0, 0.0
    for text, size, bold in runs:
        for c in text:
            if c in "\n\v":
                best, cur = max(best, cur), 0.0
                continue
            cur += char_width(c, bold) * size
    return max(best, cur)


def attr_pt(el, path, attr, scale, default=None):
    x = el.find(path, NS) if el is not None else None
    if x is None or x.get(attr) is None:
        return default
    return int(x.get(attr)) / scale


def text_body_height(txBody, width, default_size):
    """txBody(lxml)の文字の高さ(pt)・文字の数・字の大きさの [(size, n)] と、折り返さないときの最大の幅。"""
    total = 0.0
    chars = 0
    sizes = []
    widest = 0.0
    lst = txBody.find("a:lstStyle", NS)
    lvl_size = {}
    if lst is not None:
        for i in range(1, 10):
            d = lst.find("a:lvl%dpPr/a:defRPr" % i, NS)
            if d is not None and d.get("sz"):
                lvl_size[i - 1] = int(d.get("sz")) / 100
    for p in txBody.findall("a:p", NS):
        pPr = p.find("a:pPr", NS)
        lvl = int(pPr.get("lvl", "0")) if pPr is not None else 0
        psize = lvl_size.get(lvl, default_size)
        end = p.find("a:endParaRPr", NS)
        if end is not None and end.get("sz"):
            psize = int(end.get("sz")) / 100
        runs = []
        for r in p:
            tag = etree.QName(r).localname
            if tag in ("r", "fld"):
                rPr = r.find("a:rPr", NS)
                size = int(rPr.get("sz")) / 100 if rPr is not None and rPr.get("sz") else psize
                bold = rPr is not None and rPr.get("b") in ("1", "true")
                t = r.findtext("a:t", default="", namespaces=NS)
                runs.append((t, size, bold))
                n = len(re.sub(r"\s", "", t))
                chars += n
                if n:
                    sizes.append((size, n))
            elif tag == "br":
                runs.append(("\n", psize, False))
        indent = 0.0
        if pPr is not None:
            indent = int(pPr.get("marL", "0")) / EMU_PT
        if not runs:
            runs = [("", psize, False)]
        lines, _ = wrap_lines(runs, width - indent)
        widest = max(widest, widest_line(runs) + indent)
        spc_pts = attr_pt(pPr, "a:lnSpc/a:spcPts", "val", 100)
        spc_pct = attr_pt(pPr, "a:lnSpc/a:spcPct", "val", 100000)
        for s in lines:
            if spc_pts is not None:
                total += spc_pts
            else:
                total += s * SINGLE * (spc_pct if spc_pct is not None else 1.0)
        bef = attr_pt(pPr, "a:spcBef/a:spcPts", "val", 100, 0.0)
        aft = attr_pt(pPr, "a:spcAft/a:spcPts", "val", 100, 0.0)
        bef_pct = attr_pt(pPr, "a:spcBef/a:spcPct", "val", 100000)
        if bef_pct:
            bef = bef_pct * lines[0] * SINGLE
        total += bef + aft
    return total, chars, sizes, widest


def placeholder_default_size(shape):
    if not shape.is_placeholder:
        return 18.0
    try:
        t = shape.placeholder_format.type
    except Exception:  # noqa: BLE001
        return 18.0
    name = str(t)
    if "TITLE" in name and "SUB" not in name:
        return 44.0
    if "SUBTITLE" in name:
        return 32.0
    return 28.0


def leaves(shapes, transform=None):
    """(shape, (x, y, w, h) pt) を葉まで。グループの子の座標を写す。"""
    for sh in shapes:
        if sh.left is None or sh.width is None:
            if sh.shape_type is not None and sh.shape_type == 6:
                pass
            else:
                continue
        x, y, w, h = (sh.left or 0) / EMU_PT, (sh.top or 0) / EMU_PT, (sh.width or 0) / EMU_PT, (sh.height or 0) / EMU_PT
        if transform:
            x, y, w, h = transform(x, y, w, h)
        if sh.shape_type == 6:  # GROUP
            xfrm = sh._element.find("p:grpSpPr/a:xfrm", NS)
            ch_off = xfrm.find("a:chOff", NS)
            ch_ext = xfrm.find("a:chExt", NS)
            cx, cy = int(ch_off.get("x")) / EMU_PT, int(ch_off.get("y")) / EMU_PT
            cw, chh = int(ch_ext.get("cx")) / EMU_PT, int(ch_ext.get("cy")) / EMU_PT
            sx = w / cw if cw else 1.0
            sy = h / chh if chh else 1.0

            def t(a, b, c, d, x=x, y=y, cx=cx, cy=cy, sx=sx, sy=sy):
                return x + (a - cx) * sx, y + (b - cy) * sy, c * sx, d * sy

            yield from leaves(sh.shapes, t)
        else:
            yield sh, (x, y, w, h)


def visible(sh):
    el = sh._element
    tag = etree.QName(el).localname
    if tag in ("graphicFrame", "pic", "cxnSp"):
        return True
    has_text = sh.has_text_frame and sh.text_frame.text.strip() != ""
    spPr = el.find("p:spPr", NS)
    if spPr is None:
        return has_text
    no_fill = spPr.find("a:noFill", NS) is not None or (
        spPr.find("a:solidFill", NS) is None and spPr.find("a:gradFill", NS) is None
        and spPr.find("a:pattFill", NS) is None and el.find("p:style", NS) is None)
    ln = spPr.find("a:ln", NS)
    no_line = (ln is None and el.find("p:style", NS) is None) or (ln is not None and ln.find("a:noFill", NS) is not None)
    return has_text or not (no_fill and no_line)


ARROWS = ("chevron", "homePlate", "rightArrow", "pentagon")


def has_fill_or_line(sh):
    el = sh._element
    spPr = el.find("p:spPr", NS)
    if spPr is None:
        return False
    if el.find("p:style", NS) is not None and spPr.find("a:noFill", NS) is None:
        return True
    if any(spPr.find(t, NS) is not None for t in ("a:solidFill", "a:gradFill", "a:pattFill", "a:blipFill")):
        return True
    ln = spPr.find("a:ln", NS)
    return ln is not None and ln.find("a:noFill", NS) is None and len(ln)


def is_line(sh, rect):
    el = sh._element
    if etree.QName(el).localname == "cxnSp":
        return True
    geom = el.find("p:spPr/a:prstGeom", NS)
    if geom is not None and geom.get("prst") in ("line", "straightConnector1", "bentConnector2", "bentConnector3"):
        return True
    _, _, w, h = rect
    return min(w, h) < 1.5  # 罫線・細い帯


def chart_chars(sh):
    n = 0
    try:
        part = sh.chart.part
        root = etree.fromstring(part.blob)
    except Exception:  # noqa: BLE001
        return 0
    for v in root.iterfind(".//c:cat//c:v", NS):
        n += len(re.sub(r"\s", "", v.text or ""))
    for v in root.iterfind(".//c:tx//c:v", NS):
        n += len(re.sub(r"\s", "", v.text or ""))
    for t in root.iterfind(".//a:t", NS):
        n += len(re.sub(r"\s", "", t.text or ""))
    return n


def table_measure(sh, rect):
    """表の行が文字で伸びた後の高さ・文字の数・字の大きさ。"""
    tbl = sh.table
    cols = [c.width / EMU_PT for c in tbl.columns]
    height = 0.0
    chars = 0
    sizes = []
    for row in tbl.rows:
        need = row.height / EMU_PT
        ci = 0
        for cell in row.cells:
            tc = cell._tc
            span = int(tc.get("gridSpan", "1"))
            if tc.get("hMerge") or tc.get("vMerge"):
                ci += 1
                continue
            w = sum(cols[ci:ci + span])
            ci += 1
            tcPr = tc.find("a:tcPr", NS)
            ml = int(tcPr.get("marL", "91440")) / EMU_PT if tcPr is not None else 7.2
            mr = int(tcPr.get("marR", "91440")) / EMU_PT if tcPr is not None else 7.2
            mt = int(tcPr.get("marT", "45720")) / EMU_PT if tcPr is not None else 3.6
            mb = int(tcPr.get("marB", "45720")) / EMU_PT if tcPr is not None else 3.6
            body = tc.find("a:txBody", NS)
            if body is None:
                continue
            vert = tcPr is not None and tcPr.get("vert") in ("eaVert", "vert", "vert270")
            th, n, s, _ = text_body_height(body, (row.height / EMU_PT if vert else w) - ml - mr, 18.0)
            chars += n
            sizes += s
            if tc.get("rowSpan") and int(tc.get("rowSpan")) > 1:
                continue
            if not vert:
                need = max(need, th + mt + mb)
        height += need
    return max(height, rect[3]), chars, sizes


def measure_slide(slide, sw, sh_):
    items = []
    overflows = []
    shrunk = 0
    chars = 0
    sizes = []
    for sh, rect in leaves(slide.shapes):
        if not visible(sh):
            continue
        el = sh._element
        tag = etree.QName(el).localname
        has_text = False
        x, y, w, h = rect
        if tag == "graphicFrame" and sh.has_table:
            th, n, s = table_measure(sh, rect)
            chars += n
            sizes += s
            has_text = n > 0
            if th > h + TOL:
                rect = (x, y, w, th)
        elif tag == "graphicFrame" and sh.has_chart:
            chars += chart_chars(sh)
            has_text = True
        elif sh.has_text_frame:
            body = el.find(".//p:txBody", NS)
            bodyPr = body.find("a:bodyPr", NS)
            ins = {k: int(bodyPr.get(k)) / EMU_PT if bodyPr.get(k) else d
                   for k, d in (("lIns", 7.2), ("rIns", 7.2), ("tIns", 3.6), ("bIns", 3.6))}
            wrap = bodyPr.get("wrap", "square") != "none"
            inner = w - ins["lIns"] - ins["rIns"] if wrap else 1e9
            th, n, s, widest = text_body_height(body, inner, placeholder_default_size(sh))
            chars += n
            sizes += s
            has_text = n > 0
            if n:
                auto = "norm" if bodyPr.find("a:normAutofit", NS) is not None else (
                    "sp" if bodyPr.find("a:spAutoFit", NS) is not None else "")
                over_h = th + ins["tIns"] + ins["bIns"] - h
                # 折り返さない枠の字が枠より広いのは、地か枠線があるときだけはみ出し
                # (文字だけの枠なら字のある幅を矩形にして、スライドの外・重なりで見る)
                over_w = widest + ins["lIns"] + ins["rIns"] - w if not wrap and has_fill_or_line(sh) else 0.0
                if auto == "norm" and (over_h > TOL or over_w > TOL):
                    shrunk += 1
                elif auto == "sp" and over_h > TOL:
                    rect = (x, y, w, th + ins["tIns"] + ins["bIns"])  # 枠が伸びる
                if not has_fill_or_line(sh):
                    # 地も枠線も無い文字だけの枠は、字のある幅だけを要素の矩形にする(重なりと埋まり)
                    tw = min(widest, inner) + ins["lIns"] + ins["rIns"]
                    algn = (body.find("a:p/a:pPr", NS).get("algn") if body.find("a:p/a:pPr", NS) is not None else None) or "l"
                    nx = x if algn == "l" else (x + w - tw if algn == "r" else x + (w - tw) / 2)
                    rect = (nx, y, tw, rect[3])
                if auto in ("norm", "sp"):
                    pass
                elif over_h > TOL or over_w > TOL:
                    overflows.append({"kind": "text", "shape": sh.name, "text": sh.text_frame.text[:30],
                                      "over": round(max(over_h, over_w), 1)})
        x, y, w, h = rect
        if x < -TOL or y < -TOL or x + w > sw + TOL or y + h > sh_ + TOL:
            overflows.append({"kind": "slide", "shape": sh.name, "over": round(max(-x, -y, x + w - sw, y + h - sh_), 1)})
        geom = el.find("p:spPr/a:prstGeom", NS)
        items.append({"name": sh.name, "rect": rect, "text": has_text,
                      "arrow": geom is not None and geom.get("prst") in ARROWS,
                      "line": tag != "graphicFrame" and is_line(sh, rect)})
    area = sw * sh_
    solid = [it for it in items if not it["line"] and it["rect"][2] * it["rect"][3] < 0.9 * area]
    overlaps = []
    for i in range(len(solid)):
        for j in range(i + 1, len(solid)):
            a, b = solid[i]["rect"], solid[j]["rect"]
            ix = min(a[0] + a[2], b[0] + b[2]) - max(a[0], b[0])
            iy = min(a[1] + a[3], b[1] + b[3]) - max(a[1], b[1])
            if ix <= TOL or iy <= TOL:
                continue
            a_in_b = a[0] >= b[0] - TOL and a[1] >= b[1] - TOL and a[0] + a[2] <= b[0] + b[2] + TOL and a[1] + a[3] <= b[1] + b[3] + TOL
            b_in_a = b[0] >= a[0] - TOL and b[1] >= a[1] - TOL and b[0] + b[2] <= a[0] + a[2] + TOL and b[1] + b[3] <= a[1] + a[3] + TOL
            if solid[i]["arrow"] and solid[j]["arrow"]:
                continue  # 山形を噛み合わせて並べる工程の矢印
            if (not a_in_b and not b_in_a) or (solid[i]["text"] and solid[j]["text"]):
                overlaps.append([solid[i]["name"], solid[j]["name"]])
    fill = union_area([it["rect"] for it in items if it["rect"][2] * it["rect"][3] < 0.9 * area], sw, sh_) / area
    n_chars = sum(n for _, n in sizes)
    return {
        "elements": len(items),
        "chars": chars,
        "overlaps": len(overlaps),
        "overflows": len(overflows),
        "shrunk": shrunk,
        "fill": round(fill, 3),
        "minFont": min((s for s, _ in sizes), default=None),
        "meanFont": round(sum(s * n for s, n in sizes) / n_chars, 1) if n_chars else None,
        "overlapPairs": overlaps,
        "overflowItems": overflows,
    }


def union_area(rects, sw, sh_):
    """矩形の和の面積(1pt の格子で数える)。"""
    W, H = int(sw) + 1, int(sh_) + 1
    grid = bytearray(W * H)
    for x, y, w, h in rects:
        x0, y0 = max(0, int(x)), max(0, int(y))
        x1, y1 = min(W, int(x + w)), min(H, int(y + h))
        if x1 <= x0:
            continue
        row = b"\x01" * (x1 - x0)
        for yy in range(y0, y1):
            grid[yy * W + x0: yy * W + x1] = row
    return grid.count(1)


def measure(path, skip_first):
    prs = Presentation(path)
    sw, sh_ = prs.slide_width / EMU_PT, prs.slide_height / EMU_PT
    slides = []
    for i, slide in enumerate(prs.slides, 1):
        if skip_first and i == 1:
            continue
        m = measure_slide(slide, sw, sh_)
        m["slide"] = i
        slides.append(m)
    return {"file": path, "slides": slides}


def main(argv):
    skip = False
    if argv and argv[0] == "-skip-first":
        skip, argv = True, argv[1:]
    if not argv:
        print(__doc__)
        return 2
    json.dump([measure(p, skip) for p in argv], sys.stdout, ensure_ascii=False, indent=1)
    print()
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
