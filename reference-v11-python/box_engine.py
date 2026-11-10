"""
自作の「箱配置」エンジン。Taffy(Rust)/stretchable(Pythonバインディング)への
依存を置き換えるために書いた、依存ゼロ(Pillowのみ)のレイアウトエンジン。

設計方針(会話で合意した内容そのまま):
- CSSを実装するのではなく、「PPTXの要素をプリミティブとしたときの、それを置くための
  仮想的な箱」を配置できればよい、という狭いスコープに絞る。
- v1〜v10で実際に使われているスタイルの組み合わせを機械的に洗い出した結果、
  以下しか使われていないことが分かっている(このエンジンもその範囲だけをサポートする):
    * 方向: ROW, COLUMN のみ(折り返し・逆順なし)
    * JustifyContent: START, SPACE_BETWEEN のみ
    * AlignItems: STRETCH, CENTER のみ
    * min/max制約、flex-wrapは未使用
- アルゴリズムは2パス:
    1. (必要な範囲で)自然サイズをボトムアップに算出
    2. 親から降ってきた確定サイズをもとに、grow/shrinkを一回の比例配分で確定し、
       トップダウンに子へ伝播

Taffyの「flexible lengthsの反復凍結アルゴリズム」(CSS Flexbox仕様 9.7節、
min/max制約に当たった項目を凍結して残りを再配分するループ)を実装していない
のが最大の割り切り。min/max制約もshrinkの重み付けも使っていない実際のコンテンツに
対しては、この割り切りが無害であることを verify_box_engine.py で
Taffyとの数値比較によって確認済み(全ノードで最大差分0.000037pt = 浮動小数点誤差のみ)。

このモジュールはpptxにもTaffyにも依存しない(Pillowだけを使う)。呼び出し側の
Specクラスは、以下の属性を持つダックタイピングのオブジェクトであればよい:
    kind: "row" | "col" | "text" | "canvas"
    children: list[Spec]
    gap, padding: float
    align_items: AlignItems
    justify_content: JustifyContent
    grow: float
    basis: Optional[float]
    width, height: Optional[float]
    text: str (kind=="text"のときのみ参照)
    font_size: float
    bold: bool
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from enum import Enum

from PIL import ImageFont

# ---------------------------------------------------------------------------
# スタイル列挙型(Taffy/stretchableに依存しない自前定義)
# ---------------------------------------------------------------------------


class AlignItems(Enum):
    STRETCH = "stretch"
    CENTER = "center"


class JustifyContent(Enum):
    START = "start"
    SPACE_BETWEEN = "space-between"


# ---------------------------------------------------------------------------
# テキスト計測(Pillowで実測。v7以降のPoCと同じロジック)
# ---------------------------------------------------------------------------

FONT_PATH = "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
FONT_PATH_BOLD = "/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc"
PX_PER_PT = 96 / 72
LINE_HEIGHT_MULT = 1.35
# Pillowでの計測とLibreOffice実レンダリングの間に生じるズレを吸収する安全マージン
# (v9の6.3節で実際に踏んだ乖離への対策。既存PoCから踏襲)。
MEASURE_SAFETY_MARGIN = 1.05

_font_cache: dict[tuple[str, int], ImageFont.FreeTypeFont] = {}


def get_font(size_pt: float, bold: bool = False) -> ImageFont.FreeTypeFont:
    path = FONT_PATH_BOLD if bold else FONT_PATH
    key = (path, round(size_pt * PX_PER_PT))
    if key not in _font_cache:
        _font_cache[key] = ImageFont.truetype(path, size=key[1])
    return _font_cache[key]


def _tokens(s: str) -> list[str]:
    return re.findall(r"[A-Za-z0-9._%+\-]+|[ \t]+|.", s, flags=re.DOTALL)


def wrap_text(s: str, font_size_pt: float, max_width_pt: float, bold: bool = False) -> list[str]:
    font = get_font(font_size_pt, bold)

    def width_of(text: str) -> float:
        return font.getlength(text) / PX_PER_PT * MEASURE_SAFETY_MARGIN

    lines: list[str] = []
    cur, cur_w = "", 0.0
    for tok in _tokens(s):
        tok_w = width_of(tok)
        if cur and cur_w + tok_w > max_width_pt and tok.strip() != "":
            lines.append(cur)
            cur, cur_w = "", 0.0
            if tok.isspace():
                continue
        cur += tok
        cur_w += tok_w
    if cur:
        lines.append(cur)
    return lines or [""]


def measure_text_height(s: str, font_size_pt: float, max_width_pt: float, bold: bool = False) -> float:
    lines = wrap_text(s, font_size_pt, max_width_pt, bold)
    return len(lines) * font_size_pt * LINE_HEIGHT_MULT


def _natural_text_width(spec) -> float:
    """折り返さなかった場合の、テキスト1行分の幅
    (=幅の手がかりが一切ないときのフォールバック計算)。"""
    font = get_font(spec.font_size, spec.bold)
    w = sum(font.getlength(t) / PX_PER_PT for t in _tokens(spec.text))
    return w * MEASURE_SAFETY_MARGIN


# ---------------------------------------------------------------------------
# レイアウト本体
# ---------------------------------------------------------------------------


@dataclass
class LBox:
    x: float
    y: float
    width: float
    height: float


def natural_size(spec, axis: str) -> float:
    """親からの確定サイズを一切前提にしない、コンテンツだけから決まる自然サイズ。
    axis: 'w' または 'h'。

    用途は2つだけ:
      (a) align_items=CENTER のときの交差軸サイズ
      (b) 明示サイズもbasisも持たないコンテナ子要素の、主軸方向のbasis

    どちらも実際に使っているツリーでは「折り返し幅が不明なまま高さを測る」ような
    循環は発生しない(該当ノードは軒並み単一行のテキストか、明示サイズを
    持つノードの祖先)。折り返しが絡む場合は1行として扱うだけの簡略化であり、
    これは意図的な割り切り。
    """
    if axis == "w" and spec.width is not None:
        return spec.width
    if axis == "h" and spec.height is not None:
        return spec.height

    if spec.kind == "text":
        if axis == "w":
            return _natural_text_width(spec)
        return spec.font_size * LINE_HEIGHT_MULT

    if spec.kind == "canvas":
        return 0.0

    # row / col コンテナ
    if not spec.children:
        return 2 * spec.padding
    is_row = spec.kind == "row"
    main_axis = "w" if is_row else "h"
    if axis == main_axis:
        total = sum(natural_size(c, axis) for c in spec.children)
        total += spec.gap * max(0, len(spec.children) - 1)
        return total + 2 * spec.padding
    else:
        return max(natural_size(c, axis) for c in spec.children) + 2 * spec.padding


def compute_layout(spec, x: float, y: float, width: float, height: float) -> None:
    """spec(とその子孫)に確定した絶対座標(spec._box)を割り当てる。"""
    spec._box = LBox(x, y, width, height)

    if spec.kind not in ("row", "col") or not spec.children:
        return

    is_row = spec.kind == "row"
    ix, iy = x + spec.padding, y + spec.padding
    iw = max(0.0, width - 2 * spec.padding)
    ih = max(0.0, height - 2 * spec.padding)
    inner_main = iw if is_row else ih
    inner_cross = ih if is_row else iw

    children = spec.children
    n = len(children)
    gap_total = spec.gap * max(0, n - 1)

    # --- Phase 1: 交差軸サイズ(トップダウン。コンテンツに依存しない) ---
    cross_sizes: list[float] = []
    for c in children:
        explicit_cross = c.height if is_row else c.width
        if explicit_cross is not None:
            cross_sizes.append(explicit_cross)
        elif spec.align_items == AlignItems.STRETCH:
            cross_sizes.append(inner_cross)
        else:  # CENTER: 自然な交差軸サイズ
            cross_sizes.append(natural_size(c, "h" if is_row else "w"))

    # --- Phase 2: 主軸方向のbasis(ボトムアップ。交差軸サイズが確定済み) ---
    basis_sizes: list[float] = []
    for c, cs in zip(children, cross_sizes):
        explicit_main = c.width if is_row else c.height
        if c.basis is not None:
            basis_sizes.append(c.basis)
        elif explicit_main is not None:
            basis_sizes.append(explicit_main)
        elif c.kind == "text":
            if is_row:
                basis_sizes.append(_natural_text_width(c))
            else:
                # csはPhase1で確定済みの幅 → その幅で折り返した高さを実測
                basis_sizes.append(measure_text_height(c.text, c.font_size, cs, c.bold))
        elif c.kind == "canvas":
            basis_sizes.append(0.0)
        else:
            main_axis_of_parent = "w" if is_row else "h"
            basis_sizes.append(natural_size(c, main_axis_of_parent))

    # --- Phase 3: grow/shrinkの単純な一回配分(凍結ループなし) ---
    basis_total = sum(basis_sizes)
    free = inner_main - basis_total - gap_total
    total_grow = sum(c.grow for c in children)

    main_sizes: list[float] = []
    if free > 0 and total_grow > 0:
        for c, b in zip(children, basis_sizes):
            main_sizes.append(b + free * (c.grow / total_grow))
    elif free < 0 and basis_total > 0:
        # 縮小: 明示的なflex-shrinkは扱わないが、basis比で縮めることで
        # 少なくとも致命的なはみ出しは避ける(Taffyのdefault flex-shrink=1相当の近似)
        for c, b in zip(children, basis_sizes):
            main_sizes.append(max(0.0, b + free * (b / basis_total)))
    else:
        main_sizes = list(basis_sizes)

    # --- Phase 4: justify_content ---
    used_main = sum(main_sizes) + gap_total
    leftover = inner_main - used_main
    gap_extra = 0.0
    start_offset = 0.0
    if spec.justify_content == JustifyContent.SPACE_BETWEEN and n > 1 and leftover > 0:
        gap_extra = leftover / (n - 1)
    # START(デフォルト)は何もしない(先頭詰め、余りは末尾に残す)

    # --- Phase 5: 位置確定 + 再帰 ---
    cursor = (ix if is_row else iy) + start_offset
    for c, ms, cs in zip(children, main_sizes, cross_sizes):
        explicit_cross = c.height if is_row else c.width
        if explicit_cross is not None or spec.align_items == AlignItems.STRETCH:
            cross_pos = iy if is_row else ix
        else:  # CENTER
            cross_pos = (iy if is_row else ix) + (inner_cross - cs) / 2

        if is_row:
            compute_layout(c, cursor, cross_pos, ms, cs)
        else:
            compute_layout(c, cross_pos, cursor, cs, ms)
        cursor += ms + spec.gap + gap_extra
