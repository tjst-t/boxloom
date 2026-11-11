#!/usr/bin/env python3
"""Boxloom が書いた PPTX が OOXML として正しいかを、人が開く前に機械で検査する。

PowerPoint が「修復が必要です」と言う種類の誤り(Schema に合わない要素・
指す先の無い関係・内容の種類の無い部品・図形の id の重複など)を捕まえる。

使い方:
    python3 tools/validate_pptx.py [--format text|json] [--python-pptx] [--soffice] FILE.pptx ...
    python3 tools/validate_pptx.py --self-test [--soffice]

    --python-pptx  別の実装(python-pptx)で開き直し、全部の図形の文字を読めるか確かめる
    --soffice      LibreOffice で PDF に変換でき、ページの数がスライドの数と合うか確かめる
    --self-test    examples から dark/light で PPTX を作って全部が通ること、
                   それを壊したもの(19 通り)がそれぞれ決まった code で断られることを確かめる。
                   Go(PATH に go)が要る

検査すること(code):
    package               zip として読めない・[Content_Types].xml や _rels/.rels が無い・名前の重複
    xml-malformed         XML として読めない
    schema                ECMA-376 の XSD(Transitional・OPC)に合わない
    root-element          内容の種類に対して根の要素が違う
    content-type-missing  部品に内容の種類(Override か Default)が無い
    content-type-orphan   Override が無い部品を指している
    rel-target-missing    関係の指す部品が無い
    rel-target-type       関係の種類と、指す部品の内容の種類が合わない
    rel-required          スライドにレイアウトが無い、など要る関係が無い・多すぎる
    rid-missing           部品の中の r:id 等が、その部品の関係に無い
    shape-id-duplicate    1つの部品の中で図形の id(p:cNvPr の id)が重なる
    connector-target      コネクタの a:stCxn・a:endCxn が無い図形を指す
    id-duplicate          presentation の sldId・sldMasterId・sldLayoutId の id が重なる・範囲の外
    core-properties       docProps/core.xml の形が違う
    reader-python-pptx    python-pptx で開けない・読めない(--python-pptx)
    reader-soffice        LibreOffice で変換できない・ページの数が違う(--soffice)

誤りは docs/content-syntax.md の5章の形に揃える:
    {"file": "deck.pptx", "path": "/ppt/slides/slide2.xml", "code": "schema", "message": "..."}
path は PPTX の中の部品の名前。部品の中の行は message に書く。

要るもの:
    pip install lxml           (XSD の検査)
    python3 tools/fetch_ooxml_schemas.py   (XSD を tools/.ooxml-schemas/ に取ってくる。
                                            置き場所は環境変数 BOXLOOM_OOXML_SCHEMAS でも変えられる)
    pip install python-pptx    (--python-pptx のときだけ)
    LibreOffice の soffice     (--soffice のときだけ)
手順は docs/build.md の「PPTX の検査」。
"""
import copy
import io
import json
import os
import pathlib
import posixpath
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

from lxml import etree

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMA_DIR = pathlib.Path(os.environ.get("BOXLOOM_OOXML_SCHEMAS", ROOT / "tools" / ".ooxml-schemas"))

NS = {
    "p": "http://schemas.openxmlformats.org/presentationml/2006/main",
    "a": "http://schemas.openxmlformats.org/drawingml/2006/main",
    "r": "http://schemas.openxmlformats.org/officeDocument/2006/relationships",
    "ct": "http://schemas.openxmlformats.org/package/2006/content-types",
    "pr": "http://schemas.openxmlformats.org/package/2006/relationships",
    "cp": "http://schemas.openxmlformats.org/package/2006/metadata/core-properties",
    "dc": "http://purl.org/dc/elements/1.1/",
    "dcterms": "http://purl.org/dc/terms/",
    "xsi": "http://www.w3.org/2001/XMLSchema-instance",
    "ep": "http://schemas.openxmlformats.org/officeDocument/2006/extended-properties",
    "c": "http://schemas.openxmlformats.org/drawingml/2006/chart",
    "s": "http://schemas.openxmlformats.org/spreadsheetml/2006/main",
}

CT = "application/vnd.openxmlformats-officedocument."
PML = CT + "presentationml."
REL = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
SML = CT + "spreadsheetml."
XLSX = SML + "sheet"

# 内容の種類 → (Schema の名前, 根の要素)
CONTENT = {
    PML + "presentation.main+xml": ("pml", "{%s}presentation" % NS["p"]),
    PML + "slide+xml": ("pml", "{%s}sld" % NS["p"]),
    PML + "slideLayout+xml": ("pml", "{%s}sldLayout" % NS["p"]),
    PML + "slideMaster+xml": ("pml", "{%s}sldMaster" % NS["p"]),
    PML + "notesSlide+xml": ("pml", "{%s}notes" % NS["p"]),
    PML + "notesMaster+xml": ("pml", "{%s}notesMaster" % NS["p"]),
    PML + "presProps+xml": ("pml", "{%s}presentationPr" % NS["p"]),
    PML + "viewProps+xml": ("pml", "{%s}viewPr" % NS["p"]),
    PML + "tableStyles+xml": ("dml", "{%s}tblStyleLst" % NS["a"]),
    CT + "theme+xml": ("dml", "{%s}theme" % NS["a"]),
    CT + "extended-properties+xml": ("ext", "{%s}Properties" % NS["ep"]),
    CT + "drawingml.chart+xml": ("chart", "{%s}chartSpace" % NS["c"]),
    # 埋め込みの xlsx の中の部品(グラフの値の表)
    SML + "sheet.main+xml": ("sml", "{%s}workbook" % NS["s"]),
    SML + "worksheet+xml": ("sml", "{%s}worksheet" % NS["s"]),
    SML + "styles+xml": ("sml", "{%s}styleSheet" % NS["s"]),
    "application/vnd.openxmlformats-package.core-properties+xml": (None, "{%s}coreProperties" % NS["cp"]),
}

# 関係の種類 → 指す部品の内容の種類
REL_TARGET = {
    REL + "officeDocument": PML + "presentation.main+xml",
    REL + "slide": PML + "slide+xml",
    REL + "slideLayout": PML + "slideLayout+xml",
    REL + "slideMaster": PML + "slideMaster+xml",
    REL + "notesSlide": PML + "notesSlide+xml",
    REL + "notesMaster": PML + "notesMaster+xml",
    REL + "theme": CT + "theme+xml",
    REL + "presProps": PML + "presProps+xml",
    REL + "viewProps": PML + "viewProps+xml",
    REL + "tableStyles": PML + "tableStyles+xml",
    REL + "extended-properties": CT + "extended-properties+xml",
    REL + "chart": CT + "drawingml.chart+xml",
    REL + "package": XLSX,
    REL + "worksheet": SML + "worksheet+xml",
    REL + "styles": SML + "styles+xml",
    "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties":
        "application/vnd.openxmlformats-package.core-properties+xml",
}

# 部品の内容の種類 → (関係の種類, 最小, 最大)
REL_REQUIRED = {
    PML + "slide+xml": [(REL + "slideLayout", 1, 1), (REL + "notesSlide", 0, 1)],
    PML + "slideLayout+xml": [(REL + "slideMaster", 1, 1)],
    PML + "slideMaster+xml": [(REL + "theme", 1, 1)],
    PML + "notesSlide+xml": [(REL + "notesMaster", 1, 1), (REL + "slide", 1, 1)],
    PML + "notesMaster+xml": [(REL + "theme", 1, 1)],
    PML + "presentation.main+xml": [(REL + "slideMaster", 1, None), (REL + "theme", 1, 1)],
    CT + "drawingml.chart+xml": [(REL + "package", 1, 1)],
}

SHAPE_PARTS = {PML + t for t in ("slide+xml", "slideLayout+xml", "slideMaster+xml", "notesSlide+xml", "notesMaster+xml")}

_schemas = None


def schemas():
    global _schemas
    if _schemas is None:
        t, o = SCHEMA_DIR / "transitional", SCHEMA_DIR / "opc"
        if not (t / "pml.xsd").exists() or not (o / "opc-contentTypes.xsd").exists():
            sys.exit(f"OOXML の XSD が {SCHEMA_DIR} にありません。python3 tools/fetch_ooxml_schemas.py を先に流してください")
        load = lambda p: etree.XMLSchema(etree.parse(str(p)))
        _schemas = {
            "pml": load(t / "pml.xsd"),
            "dml": load(t / "dml-main.xsd"),
            "ext": load(t / "shared-documentPropertiesExtended.xsd"),
            "chart": load(t / "dml-chart.xsd"),
            "sml": load(t / "sml.xsd"),
            "ct": load(o / "opc-contentTypes.xsd"),
            "rels": load(o / "opc-relationships.xsd"),
        }
    return _schemas


def rels_name(part):
    d, b = posixpath.split(part)
    return posixpath.join(d, "_rels", b + ".rels")


def resolve(source, target):
    if target.startswith("/"):
        return posixpath.normpath(target.lstrip("/"))
    return posixpath.normpath(posixpath.join(posixpath.dirname(source), target))


class Checker:
    def __init__(self, file, main=None, prefix=""):
        self.file = file
        self.issues = []
        # main は officeDocument が指す部品の種類(PPTX は presentation、埋め込みの
        # xlsx は workbook)。prefix は埋め込みの中の部品の名前の頭(外の部品の名前+"!")。
        self.main = main or PML + "presentation.main+xml"
        self.prefix = prefix

    def err(self, part, code, message):
        path = "/" + part if part else ""
        if self.prefix:
            path = self.prefix + path
        self.issues.append({"file": self.file, "path": path, "code": code, "message": message})

    def office_forms(self, name, doc):
        """PowerPoint が書き、LibreOffice が読む形のうち、手元の XSD(ISO/IEC 29500 の
        transitional)が受け付けないものを、範囲を見てから ISO の形に直した写しを返す。
        a:buSzPct の val は ISO では "79%"、PowerPoint と ECMA-376 第1版では 1000 分の 1 %
        の整数(25000〜400000)。LibreOffice は ISO の形を読まないので、Boxloom は整数で書く。"""
        if not any(re.fullmatch(r"\d+", e.get("val", "")) for e in doc.iterfind(".//a:buSzPct", NS)):
            return doc
        doc = copy.deepcopy(doc)
        for e in doc.iterfind(".//a:buSzPct", NS):
            v = e.get("val", "")
            if not re.fullmatch(r"\d+", v):
                continue
            if not 25000 <= int(v) <= 400000:
                self.err(name, "schema", f"a:buSzPct val {v} is outside 25000..400000")
            e.set("val", f"{min(400, max(25, round(int(v) / 1000)))}%")
        return doc

    def parse(self, z, name):
        try:
            return etree.fromstring(z.read(name))
        except etree.XMLSyntaxError as e:
            self.err(name, "xml-malformed", str(e))
            return None

    def validate(self, name, doc, key):
        s = schemas()[key]
        doc = self.office_forms(name, doc)
        if not s.validate(doc):
            for e in list(s.error_log)[:5]:
                self.err(name, "schema", f"line {e.line}: {e.message}")

    def run(self, data):
        try:
            z = zipfile.ZipFile(io.BytesIO(data))
        except zipfile.BadZipFile as e:
            self.err("", "package", f"not a zip file: {e}")
            return self.issues
        names = [n for n in z.namelist() if not n.endswith("/")]
        seen = set()
        for n in names:
            if n in seen:
                self.err(n, "package", "the zip has two entries with this name")
            seen.add(n)
        parts = set(names)
        for req in ("[Content_Types].xml", "_rels/.rels"):
            if req not in parts:
                self.err(req, "package", "required part is missing")
        if "[Content_Types].xml" not in parts:
            return self.issues

        # 内容の種類
        ctdoc = self.parse(z, "[Content_Types].xml")
        if ctdoc is None:
            return self.issues
        self.validate("[Content_Types].xml", ctdoc, "ct")
        defaults = {e.get("Extension", "").lower(): e.get("ContentType") for e in ctdoc.findall("ct:Default", NS)}
        overrides = {e.get("PartName", "").lstrip("/"): e.get("ContentType") for e in ctdoc.findall("ct:Override", NS)}
        for p in overrides:
            if p not in parts:
                self.err(p, "content-type-orphan", "[Content_Types].xml has an Override for a part that does not exist")
        ctype = {}
        for p in parts - {"[Content_Types].xml"}:
            t = overrides.get(p) or defaults.get(posixpath.basename(p).rsplit(".", 1)[-1].lower() if "." in posixpath.basename(p) else "")
            if not t:
                self.err(p, "content-type-missing", "part has no content type (no Override and no Default for its extension)")
            ctype[p] = t

        # 関係
        rels = {}  # 部品 → {Id: (Type, 指す部品か None)}
        for p in sorted(parts):
            if not p.endswith(".rels"):
                continue
            doc = self.parse(z, p)
            if doc is None:
                continue
            self.validate(p, doc, "rels")
            d, b = posixpath.split(p)
            source = posixpath.join(posixpath.dirname(d), b[: -len(".rels")]) if d.endswith("_rels") else None
            if source is None or (source and source not in parts and p != "_rels/.rels"):
                self.err(p, "package", "relationships part does not belong to an existing part")
            src = "" if p == "_rels/.rels" else source
            table = {}
            for r in doc.findall("pr:Relationship", NS):
                rid, typ, tgt = r.get("Id"), r.get("Type"), r.get("Target", "")
                if r.get("TargetMode") == "External":
                    table[rid] = (typ, None)
                    continue
                full = resolve(src or "x", tgt) if src else posixpath.normpath(tgt.lstrip("/"))
                table[rid] = (typ, full)
                if full not in parts:
                    self.err(p, "rel-target-missing", f"relationship {rid} points to /{full}, which does not exist")
                    continue
                want = self.main if typ == REL + "officeDocument" else REL_TARGET.get(typ)
                got = ctype.get(full)
                if typ == REL + "image":
                    if got and not got.startswith("image/"):
                        self.err(p, "rel-target-type", f"relationship {rid} (image) points to /{full} of type {got}")
                elif want and got and want != got:
                    self.err(p, "rel-target-type", f"relationship {rid} ({typ.rsplit('/', 1)[-1]}) points to /{full} of type {got}")
            rels[src] = table
        root_types = [t for t, _ in rels.get("", {}).values()]
        if REL + "officeDocument" not in root_types:
            self.err("_rels/.rels", "rel-required", "the package has no officeDocument relationship")

        # 部品ごと
        for p in sorted(parts):
            t = ctype.get(p)
            if t not in CONTENT:
                continue
            doc = self.parse(z, p)
            if doc is None:
                continue
            key, root = CONTENT[t]
            if doc.tag != root:
                self.err(p, "root-element", f"root element is {doc.tag}, expected {root} for {t}")
                continue
            if key:
                self.validate(p, doc, key)
            else:
                self.check_core(p, doc)
            table = rels.get(p, {})
            for el in doc.iter():
                for an, av in el.attrib.items():
                    if an.startswith("{%s}" % NS["r"]) and av not in table:
                        self.err(p, "rid-missing", f"{etree.QName(el).localname} has r:{etree.QName(an).localname}=\"{av}\" but /{rels_name(p)} has no such Id")
            for typ, lo, hi in REL_REQUIRED.get(t, []):
                n = sum(1 for ty, _ in table.values() if ty == typ)
                if n < lo or (hi is not None and n > hi):
                    want = f"exactly {lo}" if lo == hi else (f"at least {lo}" if hi is None else f"{lo}..{hi}")
                    self.err(p, "rel-required", f"has {n} {typ.rsplit('/', 1)[-1]} relationship(s), expected {want}")
            if t in SHAPE_PARTS:
                self.check_shapes(p, doc)
            if t == PML + "presentation.main+xml":
                self.check_presentation(p, doc, z, parts, rels, ctype)
        # 埋め込みの xlsx(グラフの値の表)も同じ規則で中を見る。
        for p in sorted(parts):
            if ctype.get(p) == XLSX:
                inner = Checker(self.file, main=SML + "sheet.main+xml", prefix=(self.prefix or "") + "/" + p + "!")
                self.issues.extend(inner.run(z.read(p)))
        return self.issues

    def check_shapes(self, p, doc):
        ids = {}
        for c in doc.iter("{%s}cNvPr" % NS["p"]):
            i = c.get("id")
            if i in ids:
                self.err(p, "shape-id-duplicate", f"shape id {i} is used by \"{ids[i]}\" and \"{c.get('name')}\"")
            else:
                ids[i] = c.get("name")
        for tag in ("stCxn", "endCxn"):
            for c in doc.iter("{%s}%s" % (NS["a"], tag)):
                if c.get("id") not in ids:
                    self.err(p, "connector-target", f"a:{tag} points to shape id {c.get('id')}, which is not in this part")

    def check_presentation(self, p, doc, z, parts, rels, ctype):
        sld = [e.get("id") for e in doc.iterfind("p:sldIdLst/p:sldId", NS)]
        if len(sld) != len(set(sld)):
            self.err(p, "id-duplicate", f"p:sldId ids repeat: {sld}")
        big = [e.get("id") for e in doc.iterfind("p:sldMasterIdLst/p:sldMasterId", NS)]
        for (src, table) in rels.items():
            if ctype.get(src) == PML + "slideMaster+xml":
                try:
                    m = etree.fromstring(z.read(src))
                except etree.XMLSyntaxError:
                    continue
                big += [e.get("id") for e in m.iterfind("p:sldLayoutIdLst/p:sldLayoutId", NS)]
        if len(big) != len(set(big)):
            self.err(p, "id-duplicate", f"slide master and layout ids must be unique together: {big}")
        for i in big:
            if i is not None and not (2147483648 <= int(i) <= 4294967295):
                self.err(p, "id-duplicate", f"slide master/layout id {i} is outside 2147483648..4294967295")

    def check_core(self, p, doc):
        for child in doc:
            if child.tag in ("{%s}created" % NS["dcterms"], "{%s}modified" % NS["dcterms"]):
                if child.get("{%s}type" % NS["xsi"]) != "dcterms:W3CDTF":
                    self.err(p, "core-properties", f"{etree.QName(child).localname} must have xsi:type=\"dcterms:W3CDTF\"")
                elif not re.fullmatch(r"\d{4}(-\d\d(-\d\d(T\d\d:\d\d(:\d\d(\.\d+)?)?(Z|[+-]\d\d:\d\d))?)?)?", (child.text or "").strip()):
                    self.err(p, "core-properties", f"{etree.QName(child).localname} is not a W3CDTF date: {child.text!r}")
            elif etree.QName(child).namespace not in (NS["cp"], NS["dc"], NS["dcterms"]):
                self.err(p, "core-properties", f"unexpected element {child.tag}")


def read_with_python_pptx(path, file, issues):
    try:
        from pptx import Presentation
    except ImportError:
        sys.exit("--python-pptx には pip install python-pptx が要ります")
    try:
        prs = Presentation(path)
        for s in prs.slides:
            for sh in s.shapes:
                if sh.has_text_frame:
                    _ = sh.text_frame.text
            if s.has_notes_slide:
                _ = s.notes_slide.notes_text_frame.text if s.notes_slide.notes_text_frame else ""
        return len(prs.slides)
    except Exception as e:  # noqa: BLE001 — どんな失敗も「読めない」として返す
        issues.append({"file": file, "path": "", "code": "reader-python-pptx", "message": f"{type(e).__name__}: {e}"})
        return None


def slide_count(data):
    try:
        z = zipfile.ZipFile(io.BytesIO(data))
        return len(etree.fromstring(z.read("ppt/presentation.xml")).findall("p:sldIdLst/p:sldId", NS))
    except Exception:  # noqa: BLE001
        return None


def read_with_soffice(path, file, data, issues):
    soffice = shutil.which("soffice") or shutil.which("libreoffice")
    if not soffice:
        sys.exit("--soffice には LibreOffice(soffice)が要ります")
    with tempfile.TemporaryDirectory() as d:
        r = subprocess.run([soffice, f"-env:UserInstallation=file://{d}/profile", "--headless", "--convert-to", "pdf", "--outdir", d, path],
                           capture_output=True, text=True, timeout=180)
        pdf = pathlib.Path(d) / (pathlib.Path(path).stem + ".pdf")
        if r.returncode != 0 or not pdf.exists():
            issues.append({"file": file, "path": "", "code": "reader-soffice", "message": f"conversion failed: {r.stderr.strip()[:300]}"})
            return
        pages = len(re.findall(rb"/Type\s*/Page[^s]", pdf.read_bytes()))
        want = slide_count(data)
        if want is not None and pages != want:
            issues.append({"file": file, "path": "", "code": "reader-soffice", "message": f"PDF has {pages} pages, the deck has {want} slides"})


def check_file(path, python_pptx=False, soffice=False, data=None, name=None):
    data = data if data is not None else pathlib.Path(path).read_bytes()
    file = name or str(path)
    issues = Checker(file).run(data)
    if python_pptx and not issues:
        read_with_python_pptx(io.BytesIO(data), file, issues)
    if soffice and not issues:
        read_with_soffice(str(path), file, data, issues)
    return issues


def fmt_text(i):
    return f"{i['file']} {i['path'] or '/'}: {i['code']}: {i['message']}"


# --- 自分の試験 ---

def rewrite(data, edit):
    """zip の部品を edit(dict 名前→bytes)で変えた写しを返す。値が None なら消す。"""
    src = zipfile.ZipFile(io.BytesIO(data))
    files = {n: src.read(n) for n in src.namelist()}
    edit(files)
    out = io.BytesIO()
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        for n, b in files.items():
            if b is not None:
                z.writestr(n, b)
    return out.getvalue()


def sub(name, old, new, count=1):
    def f(files):
        s = files[name].decode()
        if old not in s:
            raise AssertionError(f"mutation target not found in {name}: {old[:60]}")
        files[name] = s.replace(old, new, count).encode()
    return f


def sub_embedded(name, inner, old, new):
    """埋め込みの部品(xlsx)の中の部品 inner を書き換える。"""
    def f(files):
        files[name] = rewrite(files[name], sub(inner, old, new))
    return f


def first_match(data, name, pattern):
    s = zipfile.ZipFile(io.BytesIO(data)).read(name).decode()
    m = re.search(pattern, s)
    if not m:
        raise AssertionError(f"pattern not found in {name}: {pattern}")
    return m.group(0)


def mutations(shapes, notes, blocks, charts):
    """(名前, 元の PPTX, 変え方, 期待する code)。blocks は表と画像を持つ文書(examples/ir/blocks.json)、charts はグラフを持つ文書(examples/ir/charts.json)。"""
    xlsx = "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx"
    s2 = "ppt/slides/slide2.xml"
    first_id = first_match(shapes, s2, r'<p:cNvPr id="\d+"')
    other_id = re.findall(r'<p:cNvPr id="(\d+)"', zipfile.ZipFile(io.BytesIO(shapes)).read(s2).decode())[1]
    stcxn = first_match(shapes, s2, r'<a:stCxn id="\d+"')
    pic = next(n for n in zipfile.ZipFile(io.BytesIO(blocks)).namelist() if n.startswith("ppt/slides/slide") and "<p:pic>" in zipfile.ZipFile(io.BytesIO(blocks)).read(n).decode())
    tbl = next(n for n in zipfile.ZipFile(io.BytesIO(blocks)).namelist() if n.startswith("ppt/slides/slide") and "<a:tbl>" in zipfile.ZipFile(io.BytesIO(blocks)).read(n).decode())
    media = next(n for n in zipfile.ZipFile(io.BytesIO(blocks)).namelist() if n.startswith("ppt/media/") and n.endswith(".png"))
    embed = first_match(blocks, pic, r'r:embed="[^"]+"')
    return [
        ("malformed XML", shapes, sub(s2, "</p:sld>", "</p:sldx>"), "xml-malformed"),
        ("unknown element in a shape tree", shapes, sub(s2, "</p:spTree>", "<p:bogus/></p:spTree>"), "schema"),
        ("bad attribute value", shapes, sub(s2, 'prst="', 'prst="notAShape', 1), "schema"),
        ("bullet size out of range (PowerPoint's integer form)", shapes, sub(s2, "</a:lnSpc></a:pPr>", '</a:lnSpc><a:buSzPct val="500000"/></a:pPr>'), "schema"),
        ("slide part removed", shapes, lambda f: f.__setitem__(s2, None), "rel-target-missing"),
        ("Default for .rels removed", shapes, lambda f: sub("[Content_Types].xml", first_match(shapes, "[Content_Types].xml", r'<Default Extension="rels"[^>]*/>'), "")(f), "content-type-missing"),
        ("Override removed (slide falls back to application/xml)", shapes, sub("[Content_Types].xml", '<Override PartName="/ppt/slides/slide2.xml"', '<Override PartName="/ppt/slides/slide2.xmlx"'), "rel-target-type"),
        ("Override for a missing part", shapes, sub("[Content_Types].xml", "</Types>", '<Override PartName="/ppt/slides/slide99.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>'), "content-type-orphan"),
        ("r:id without relationship", shapes, sub("ppt/presentation.xml", 'r:id="rId2"', 'r:id="rId99"'), "rid-missing"),
        ("duplicate shape id", shapes, sub(s2, f'<p:cNvPr id="{other_id}"', first_id), "shape-id-duplicate"),
        ("connector to a missing shape", shapes, sub(s2, stcxn, '<a:stCxn id="9999"'), "connector-target"),
        ("duplicate slide id", shapes, sub("ppt/presentation.xml", '<p:sldId id="257"', '<p:sldId id="256"'), "id-duplicate"),
        ("slide without layout", shapes, sub("ppt/slides/_rels/slide2.xml.rels", "relationships/slideLayout", "relationships/notAType"), "rel-required"),
        ("relationship to the wrong kind of part", shapes, sub("ppt/slides/_rels/slide2.xml.rels", first_match(shapes, "ppt/slides/_rels/slide2.xml.rels", r"\.\./slideLayouts/slideLayout\d+\.xml"), "../theme/theme1.xml"), "rel-target-type"),
        ("Content_Types removed", notes, lambda f: f.__setitem__("[Content_Types].xml", None), "package"),
        ("notes slide pointing to a missing master", notes, sub("ppt/notesSlides/_rels/notesSlide1.xml.rels", "notesMaster1.xml", "notesMaster9.xml"), "rel-target-missing"),
        ("picture's media part removed", blocks, lambda f: f.__setitem__(media, None), "rel-target-missing"),
        ("Default for .png removed", blocks, lambda f: sub("[Content_Types].xml", '<Default Extension="png" ContentType="image/png"/>', "")(f), "content-type-missing"),
        ("picture r:embed without relationship", blocks, sub(pic, embed, 'r:embed="rId99"'), "rid-missing"),
        ("unknown element in a table cell", blocks, sub(tbl, "<a:tcPr", "<a:bogus/><a:tcPr"), "schema"),
        ("unknown element in a chart", charts, sub("ppt/charts/chart1.xml", "</c:plotArea>", "<c:bogus/></c:plotArea>"), "schema"),
        ("chart without its workbook", charts, sub("ppt/charts/_rels/chart1.xml.rels", "relationships/package", "relationships/notAType"), "rel-required"),
        ("embedded workbook removed", charts, lambda f: f.__setitem__(xlsx, None), "rel-target-missing"),
        ("unknown element in the embedded sheet", charts, sub_embedded(xlsx, "xl/worksheets/sheet1.xml", "</sheetData>", "</sheetData><bogus/>"), "schema"),
    ]


def build_examples(outdir):
    """examples から dark/light で PPTX を作る。従来のデモ(go run ./cmd/boxloom)も。"""
    go = shutil.which("go") or "/usr/local/go/bin/go"
    gdir = ROOT / "boxloom"
    srcs = sorted((ROOT / "examples" / "ir").glob("*.json")) + sorted((ROOT / "examples" / "md").glob("*.bx.md")) \
        + sorted((ROOT / "examples" / "ir" / "check-warnings").glob("*.json")) + sorted((ROOT / "examples" / "md" / "warnings").glob("*.bx.md"))
    out = []
    for src in srcs:
        for tpl in ("dark", "light"):
            dst = outdir / f"{src.parent.name}-{src.name}-{tpl}.pptx"
            r = subprocess.run([go, "run", "./cmd/boxloom", "build", "-template", tpl, "-o", str(dst), str(src)], cwd=gdir, capture_output=True, text=True)
            if r.returncode != 0:
                sys.exit(f"build failed for {src} ({tpl}):\n{r.stderr}")
            out.append(dst)
    demo = outdir / "demo"
    demo.mkdir()
    # 従来のデモは out/ に書くので、作業用の写しの中で流す
    tmpgo = outdir / "go"
    shutil.copytree(gdir, tmpgo, ignore=shutil.ignore_patterns("out"))
    (tmpgo / "out").mkdir()
    r = subprocess.run([go, "run", "./cmd/boxloom"], cwd=tmpgo, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"go run ./cmd/boxloom failed:\n{r.stderr}")
    out += sorted((tmpgo / "out").glob("*.pptx"))
    return out


def self_test(soffice):
    have_pptx = True
    try:
        import pptx  # noqa: F401
    except ImportError:
        have_pptx = False
    ok = True
    with tempfile.TemporaryDirectory() as d:
        decks = build_examples(pathlib.Path(d))
        for p in decks:
            issues = check_file(p, python_pptx=have_pptx, soffice=soffice)
            print(("PASS" if not issues else "FAIL") + f"  valid   {p.name}")
            for i in issues:
                print("      " + fmt_text(i))
            ok &= not issues
        shapes = next(p for p in decks if p.name == "ir-shapes.json-light.pptx").read_bytes()
        notes = next(p for p in decks if p.name == "ir-speaker-notes.json-light.pptx").read_bytes()
        charts = next(p for p in decks if p.name == "ir-charts.json-light.pptx").read_bytes()
        blocks = next(p for p in decks if p.name == "ir-blocks.json-light.pptx").read_bytes()
        for name, base, edit, code in mutations(shapes, notes, blocks, charts):
            issues = check_file(None, data=rewrite(base, edit), name=name)
            codes = {i["code"] for i in issues}
            hit = code in codes
            print(("PASS" if hit else "FAIL") + f"  broken  {name} -> " + (", ".join(sorted(codes)) or "no issues") + f" (want {code})")
            ok &= hit
    if not have_pptx:
        print("note: python-pptx が無いので、別の実装で開き直す確かめは飛ばした")
    return 0 if ok else 1


def main(argv):
    fmt, python_pptx, soffice, files, selftest = "text", False, False, [], False
    it = iter(argv)
    for a in it:
        if a == "--format":
            fmt = next(it, "text")
        elif a == "--python-pptx":
            python_pptx = True
        elif a == "--soffice":
            soffice = True
        elif a == "--self-test":
            selftest = True
        elif a in ("-h", "--help"):
            print(__doc__)
            return 0
        else:
            files.append(a)
    if selftest:
        return self_test(soffice)
    if not files:
        print(__doc__, file=sys.stderr)
        return 2
    all_issues = []
    for f in files:
        all_issues += check_file(f, python_pptx=python_pptx, soffice=soffice)
    if fmt == "json":
        json.dump(all_issues, sys.stdout, ensure_ascii=False, indent=2)
        print()
    else:
        for i in all_issues:
            print(fmt_text(i))
        print(f"{len(files)} file(s), {len(all_issues)} error(s)")
    return 1 if all_issues else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
