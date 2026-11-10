"""
宣言的PPTX生成 PoC v11: v10からTaffy(Rust製レイアウトエンジン)への依存を
完全に取り除き、自作の箱配置エンジン(box_engine.py)だけで組版する。
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from datetime import date
from typing import Callable, Literal, Optional

from pptx import Presentation
from pptx.dml.color import RGBColor
from pptx.enum.dml import MSO_LINE_DASH_STYLE
from pptx.enum.shapes import MSO_CONNECTOR, MSO_SHAPE
from pptx.enum.text import MSO_ANCHOR, PP_ALIGN
from pptx.util import Emu, Pt

from box_engine import (
    LINE_HEIGHT_MULT,
    MEASURE_SAFETY_MARGIN,
    PX_PER_PT,
    AlignItems,
    JustifyContent,
    LBox,
    compute_layout as _compute_layout,
    get_font,
    measure_text_height,
    wrap_text,
)

SPACE = {"xs": 4, "sm": 8, "md": 16, "lg": 24, "xl": 32, "xxl": 48}
TYPE = {"title": 30, "h2": 16, "stat": 34, "body": 12.5, "caption": 10.5, "micro": 9, "nano": 8}

FONT_NAME = "Noto Sans CJK JP"

EMU_PER_PT = 12700
SLIDE_W_PT = 960.0
SLIDE_H_PT = 540.0

THEME_DARK = {
    "bg": "12151C", "card": "1C212C", "card_alt": "232939",
    "text_primary": "F5F7FA", "text_muted": "97A1B3", "grid": "2E3444",
    "support": "34C08F", "conditional": "4C8DFF", "none": "3A4150",
    "danger": "E0555A", "risk": "6B3540", "accent": "F2A93B",
    "on_conditional": "F5F7FA",
}

THEME_LIGHT = {
    "bg": "FFFFFF", "card": "F7F9FD", "card_alt": "E7EDFA",
    "text_primary": "12203D", "text_muted": "5C6B85", "grid": "D7E0F2",
    "support": "1E9E6B", "conditional": "0033CC", "none": "AAB4C6",
    "danger": "C0293C", "risk": "F4D9DE", "accent": "0033CC",
    "on_conditional": "FFFFFF",
}

def rgb(hex_color: str) -> RGBColor:
    return RGBColor.from_string(hex_color)


def pt_to_emu(v: float) -> Emu:
    return Emu(round(v * EMU_PER_PT))


@dataclass
class Spec:
    kind: Literal["row", "col", "text", "canvas"]
    children: list["Spec"] = field(default_factory=list)

    gap: float = 0
    padding: float = 0
    background: Optional[str] = None
    radius: Optional[float] = None
    align_items: AlignItems = AlignItems.STRETCH
    justify_content: JustifyContent = JustifyContent.START
    grow: float = 0
    basis: Optional[float] = None
    width: Optional[float] = None
    height: Optional[float] = None

    text: str = ""
    font_size: float = TYPE["body"]
    color: str = "F5F7FA"
    bold: bool = False
    align: PP_ALIGN = PP_ALIGN.LEFT
    anchor: MSO_ANCHOR = MSO_ANCHOR.TOP

    canvas_render: Optional[Callable[[object, "Box"], None]] = None

    _box: Optional[LBox] = field(default=None, repr=False, compare=False)


def row(children, **kw) -> Spec:
    return Spec(kind="row", children=children, **kw)


def col(children, **kw) -> Spec:
    return Spec(kind="col", children=children, **kw)


def text(content: str, **kw) -> Spec:
    return Spec(kind="text", text=content, **kw)


def canvas(render: Callable[[object, "Box"], None], **kw) -> Spec:
    return Spec(kind="canvas", canvas_render=render, **kw)


def emit(spec: Spec, slide) -> None:
    box = spec._box
    left, top = pt_to_emu(box.x), pt_to_emu(box.y)
    width, height = pt_to_emu(box.width), pt_to_emu(box.height)

    if spec.kind in ("row", "col", "canvas") and spec.background is not None:
        shape_type = MSO_SHAPE.ROUNDED_RECTANGLE if spec.radius else MSO_SHAPE.RECTANGLE
        shape = slide.shapes.add_shape(shape_type, left, top, width, height)
        shape.fill.solid()
        shape.fill.fore_color.rgb = rgb(spec.background)
        shape.line.fill.background()
        shape.shadow.inherit = False
        if spec.radius:
            try:
                shape.adjustments[0] = min(0.5, spec.radius / min(box.width, box.height))
            except (IndexError, ZeroDivisionError):
                pass

    if spec.kind == "text":
        tb = slide.shapes.add_textbox(left, top, width, height)
        tf = tb.text_frame
        tf.word_wrap = True
        tf.vertical_anchor = spec.anchor
        tf.margin_left = tf.margin_right = tf.margin_top = tf.margin_bottom = 0
        p = tf.paragraphs[0]
        p.alignment = spec.align
        run = p.add_run()
        run.text = spec.text
        run.font.name = FONT_NAME
        run.font.size = Pt(spec.font_size)
        run.font.bold = spec.bold
        run.font.color.rgb = rgb(spec.color)

    if spec.kind == "canvas" and spec.canvas_render is not None:
        spec.canvas_render(slide, box)

    for child in spec.children:
        emit(child, slide)


def content_fill_ratio(root_spec: Spec) -> float:
    root_box = root_spec._box
    available_h = root_box.height - 2 * root_spec.padding

    def deepest_bottom(spec: Spec) -> float:
        box = spec._box
        bottom = (box.y - root_box.y) + box.height
        for c in spec.children:
            bottom = max(bottom, deepest_bottom(c))
        return bottom

    used_h = max(deepest_bottom(c) for c in root_spec.children) - root_spec.padding
    return used_h / available_h


FILL_RATIO_MIN = 0.65
FILL_RATIO_MAX = 1.00
FLOAT_EPS = 0.01


def lint_fill_ratio(ratio: float) -> Optional[str]:
    if ratio > FILL_RATIO_MAX + FLOAT_EPS:
        return f"[FILL-RATIO WARNING] overflow: {ratio:.1%} (> {FILL_RATIO_MAX:.0%})"
    if ratio < FILL_RATIO_MIN:
        return f"[FILL-RATIO WARNING] underfilled: {ratio:.1%} (< {FILL_RATIO_MIN:.0%})"
    return None


def emit_fill_ratio_notes(prs_slide, warning: Optional[str]) -> None:
    if not warning:
        return
    notes = prs_slide.notes_slide
    notes.notes_text_frame.text = warning


@dataclass
class TimeScale:
    control_points: list[tuple[date, float]]

    def frac(self, d: date) -> float:
        pts = self.control_points
        if d <= pts[0][0]:
            return pts[0][1]
        if d >= pts[-1][0]:
            return pts[-1][1]
        for (d0, f0), (d1, f1) in zip(pts, pts[1:]):
            if d0 <= d <= d1:
                span = (d1 - d0).days or 1
                t = (d - d0).days / span
                return f0 + t * (f1 - f0)
        return pts[-1][1]

    def x(self, d: date, x0: float, width: float) -> float:
        return x0 + self.frac(d) * width


@dataclass
class GanttSegment:
    start: date
    end: date
    role: str
    label: str = ""
    open_end: bool = False


@dataclass
class GanttRow:
    label: str
    segments: list[GanttSegment] = field(default_factory=list)
    sub_group: Optional[str] = None
    note: Optional[tuple[date, date, str]] = None


@dataclass
class GanttGroup:
    label: str
    rows: list[GanttRow]
    risk_band: Optional[tuple[date, date]] = None


@dataclass
class Annotation:
    anchor_date: date
    anchor_y_frac: float
    text_: str
    role: str
    direction: Literal["up", "down"] = "down"
    emphasis: bool = False


@dataclass
class GanttChart:
    scale: TimeScale
    ticks: list[tuple[date, str, str, str]]
    groups: list[GanttGroup]
    legend: list[tuple[str, str]]
    milestones: list[tuple[date, str]] = field(default_factory=list)
    annotations: list[Annotation] = field(default_factory=list)
    break_before: Optional[date] = None
    extension_note: Optional[tuple[date, float, str, str]] = None


class Box:
    def __init__(self, x: float, y: float, width: float, height: float):
        self.x, self.y, self.width, self.height = x, y, width, height


def _add_text(
    slide, x, y, w, h, s, size, color, bold=False, align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP, wrap=True
):
    tb = slide.shapes.add_textbox(pt_to_emu(x), pt_to_emu(y), pt_to_emu(w), pt_to_emu(h))
    tf = tb.text_frame
    tf.word_wrap = wrap
    tf.vertical_anchor = anchor
    tf.margin_left = tf.margin_right = tf.margin_top = tf.margin_bottom = 0
    p = tf.paragraphs[0]
    p.alignment = align
    run = p.add_run()
    run.text = s
    run.font.name = FONT_NAME
    run.font.size = Pt(size)
    run.font.bold = bold
    run.font.color.rgb = rgb(color)
    return tb


def _add_rect(slide, x, y, w, h, color=None, radius=None, line_color=None, line_dash=None, line_width=0.75):
    shape_type = MSO_SHAPE.ROUNDED_RECTANGLE if radius else MSO_SHAPE.RECTANGLE
    shape = slide.shapes.add_shape(shape_type, pt_to_emu(x), pt_to_emu(y), pt_to_emu(w), pt_to_emu(h))
    if color is not None:
        shape.fill.solid()
        shape.fill.fore_color.rgb = rgb(color)
    else:
        shape.fill.background()
    if line_color:
        shape.line.color.rgb = rgb(line_color)
        shape.line.width = Pt(line_width)
        if line_dash:
            shape.line.dash_style = line_dash
    else:
        shape.line.fill.background()
    shape.shadow.inherit = False
    if radius:
        try:
            shape.adjustments[0] = min(0.5, radius / min(w, h))
        except (IndexError, ZeroDivisionError):
            pass
    return shape


def _add_line(slide, x1, y1, x2, y2, color, width=1.0, dash=None):
    conn = slide.shapes.add_connector(
        MSO_CONNECTOR.STRAIGHT, pt_to_emu(x1), pt_to_emu(y1), pt_to_emu(x2), pt_to_emu(y2)
    )
    conn.line.color.rgb = rgb(color)
    conn.line.width = Pt(width)
    if dash:
        conn.line.dash_style = dash
    return conn


def _add_dot(slide, cx, cy, r, color):
    shape = slide.shapes.add_shape(
        MSO_SHAPE.OVAL, pt_to_emu(cx - r), pt_to_emu(cy - r), pt_to_emu(r * 2), pt_to_emu(r * 2)
    )
    shape.fill.solid()
    shape.fill.fore_color.rgb = rgb(color)
    shape.line.fill.background()
    shape.shadow.inherit = False
    return shape


def _rects_overlap(a, b, margin=4.0) -> bool:
    ax, ay, aw, ah = a
    bx, by, bw, bh = b
    return not (
        ax + aw + margin < bx or bx + bw + margin < ax or ay + ah + margin < by or by + bh + margin < ay
    )


GUTTER_GROUP_W = 70.0
GUTTER_SUB_W = 46.0
GUTTER_LABEL_W = 118.0
LEGEND_H = 16.0
MILESTONE_H = 14.0
AXIS_LABEL_H = 24.0
GROUP_GAP = 8.0
MIN_ROW_H = 17.0

ANN_BOX_W = 176.0
ANN_BOX_H = 38.0
ANN_LANE_GAP = 8.0


def render_gantt(slide, box: Box, chart: GanttChart, theme: dict) -> None:
    def c(role: str) -> str:
        return theme[role]

    pad = SPACE["md"]
    x0, y0 = box.x + pad, box.y + pad
    w, h = box.width - 2 * pad, box.height - 2 * pad

    has_up = any(a.direction == "up" for a in chart.annotations)
    has_down = any(a.direction == "down" for a in chart.annotations)
    ann_top_h = (ANN_BOX_H + ANN_LANE_GAP) if has_up else 0.0
    ann_bottom_h = (ANN_BOX_H + ANN_LANE_GAP) if has_down else 0.0
    milestone_h = (MILESTONE_H + 4) if chart.milestones else 0.0

    lx = x0
    for role, label in chart.legend:
        _add_rect(slide, lx, y0 + 2, 15, 7, c(role), radius=2, line_color=c("text_muted"), line_width=0.5)
        label_w = get_font(TYPE["micro"]).getlength(label) / PX_PER_PT * MEASURE_SAFETY_MARGIN
        _add_text(slide, lx + 19, y0 - 2, label_w + 4, LEGEND_H, label, TYPE["micro"], c("text_muted"), wrap=False)
        lx += 19 + 6 + label_w + SPACE["md"]

    up_y = y0 + LEGEND_H + SPACE["xs"]
    milestone_y = up_y + ann_top_h
    axis_label_y = milestone_y + milestone_h
    axis_y = axis_label_y + AXIS_LABEL_H + 5
    gutter_w = GUTTER_GROUP_W + GUTTER_SUB_W + GUTTER_LABEL_W
    chart_x0 = x0 + gutter_w
    chart_w = w - gutter_w
    body_top = axis_y + 5
    body_h = (y0 + h) - ann_bottom_h - body_top

    scale = chart.scale

    def sx(d: date) -> float:
        return scale.x(d, chart_x0, chart_w)

    for d, label in chart.milestones:
        mx = sx(d)
        _add_text(
            slide, mx - 74, milestone_y, 148, MILESTONE_H, "★ " + label, TYPE["nano"],
            c("accent"), bold=True, align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, wrap=False,
        )

    for d, main_label, sub_label, role in chart.ticks:
        tx = sx(d)
        _add_text(
            slide, tx - 32, axis_label_y, 64, 13, main_label, TYPE["micro"], c(role),
            bold=True, align=PP_ALIGN.CENTER, wrap=False,
        )
        if sub_label:
            _add_text(
                slide, tx - 40, axis_label_y + 12, 80, 12, sub_label, TYPE["nano"],
                c("text_muted"), align=PP_ALIGN.CENTER, wrap=False,
            )
        _add_dot(slide, tx, axis_y, 2.2, c(role))
        _add_line(slide, tx, body_top, tx, body_top + body_h, c("grid"), width=0.75, dash=MSO_LINE_DASH_STYLE.DASH)

    _add_line(slide, chart_x0, axis_y, x0 + w, axis_y, c("text_muted"), width=1.25)

    if chart.break_before is not None:
        pts = scale.control_points
        bx = x0 + w
        for (d0, f0), (d1, f1) in zip(pts, pts[1:]):
            if d1 == chart.break_before:
                bx = chart_x0 + ((f0 + f1) / 2) * chart_w
                break
        for dx in (-3.0, 3.0):
            _add_line(slide, bx + dx - 3, axis_y - 7, bx + dx + 3, axis_y + 7, c("bg"), width=3.0)
            _add_line(slide, bx + dx - 3, axis_y - 7, bx + dx + 3, axis_y + 7, c("text_primary"), width=1.0)

    total_rows = sum(len(g.rows) for g in chart.groups)
    n_groups = len(chart.groups)
    row_h = max(MIN_ROW_H, (body_h - GROUP_GAP * (n_groups - 1)) / total_rows)

    sub_x = x0 + GUTTER_GROUP_W
    label_x = sub_x + GUTTER_SUB_W

    placed_annotation_boxes: list[tuple[float, float, float, float]] = []
    y_cursor = body_top
    for gi, group in enumerate(chart.groups):
        group_top = y_cursor
        group_h = row_h * len(group.rows)

        if group.risk_band:
            rb0, rb1 = group.risk_band
            _add_rect(slide, sx(rb0), group_top, sx(rb1) - sx(rb0), group_h, c("risk"))

        _add_rect(slide, x0, group_top, GUTTER_GROUP_W - 4, group_h, c("card_alt"), radius=6)
        _add_text(
            slide, x0 + 3, group_top, GUTTER_GROUP_W - 10, group_h, group.label, TYPE["nano"],
            c("text_primary"), bold=True, align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
        )

        row_tops = [group_top + i * row_h for i in range(len(group.rows))]

        i = 0
        while i < len(group.rows):
            sub = group.rows[i].sub_group
            j = i
            while j + 1 < len(group.rows) and group.rows[j + 1].sub_group == sub:
                j += 1
            if sub:
                sub_top = row_tops[i]
                sub_h = row_tops[j] + row_h - sub_top
                _add_text(
                    slide, sub_x + 2, sub_top, GUTTER_SUB_W - 6, sub_h, sub, TYPE["nano"],
                    c("text_muted"), align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                )
            i = j + 1

        for idx, row_spec in enumerate(group.rows):
            row_top = row_tops[idx]
            _add_text(
                slide, label_x, row_top, GUTTER_LABEL_W - 8, row_h, row_spec.label,
                TYPE["micro"], c("text_muted"), anchor=MSO_ANCHOR.MIDDLE, wrap=False,
            )

            bar_h = row_h - 6
            bar_y = row_top + (row_h - bar_h) / 2

            if row_spec.note:
                n0, n1, ntext = row_spec.note
                nx1, nx2 = sx(n0), sx(n1)
                _add_rect(
                    slide, nx1, bar_y, max(nx2 - nx1, 1.0), bar_h, color=None, radius=3,
                    line_color=c("text_muted"), line_dash=MSO_LINE_DASH_STYLE.DASH,
                )
                _add_text(
                    slide, nx1 + 5, bar_y, nx2 - nx1 - 10, bar_h, ntext, TYPE["nano"],
                    c("text_muted"), anchor=MSO_ANCHOR.MIDDLE,
                )
            else:
                for seg in row_spec.segments:
                    bx1, bx2 = sx(seg.start), sx(seg.end)
                    seg_color = c(seg.role)
                    _add_rect(slide, bx1, bar_y, max(bx2 - bx1, 1.0), bar_h, seg_color, radius=3)
                    if seg.label and (bx2 - bx1) > 46:
                        label_color = c("on_conditional") if seg.role == "conditional" else c("text_primary")
                        _add_text(
                            slide, bx1 + 5, bar_y, bx2 - bx1 - 8, bar_h, seg.label, TYPE["nano"],
                            label_color,
                            anchor=MSO_ANCHOR.MIDDLE, wrap=False,
                        )
                    if seg.open_end:
                        for dx in (5, 11, 17):
                            _add_dot(slide, bx2 + dx, bar_y + bar_h / 2, 1.4, seg_color)

        y_cursor += group_h
        if gi < n_groups - 1:
            _add_line(slide, x0, y_cursor + GROUP_GAP / 2, x0 + w, y_cursor + GROUP_GAP / 2, c("grid"), width=0.75)
            y_cursor += GROUP_GAP

    if chart.extension_note:
        sdate, yfrac, label, role = chart.extension_note
        ex0 = sx(sdate)
        ey = body_top + yfrac * body_h
        _add_line(slide, ex0, ey, x0 + w, ey, c(role), width=1.25, dash=MSO_LINE_DASH_STYLE.DASH)
        _add_text(
            slide, ex0, ey - 15, (x0 + w) - ex0, 13, label, TYPE["nano"], c(role),
            bold=True, align=PP_ALIGN.LEFT, wrap=False,
        )

    box_w, box_h = ANN_BOX_W, ANN_BOX_H
    down_y = body_top + body_h + ANN_LANE_GAP
    for ann in chart.annotations:
        ax = sx(ann.anchor_date)
        ay = body_top + ann.anchor_y_frac * body_h
        ann_color = c(ann.role)

        if ann.emphasis:
            _add_line(slide, ax, body_top, ax, body_top + body_h, ann_color, width=2.0)

        _add_dot(slide, ax, ay, 3.0, ann_color)

        bx = min(max(ax - box_w / 2, x0), x0 + w - box_w)
        by = up_y if ann.direction == "up" else down_y
        step = box_w + 8
        tries = 0
        while any(_rects_overlap((bx, by, box_w, box_h), b) for b in placed_annotation_boxes) and tries < 6:
            bx = min(max(bx + step, x0), x0 + w - box_w)
            tries += 1
        placed_annotation_boxes.append((bx, by, box_w, box_h))

        _add_rect(slide, bx, by, box_w, box_h, c("card_alt"), radius=8, line_color=ann_color, line_width=1.25)
        _add_text(
            slide, bx + 8, by + 4, box_w - 16, box_h - 8, ann.text_, TYPE["nano"] + 0.5,
            c("text_primary"),
        )
        conn_y = by + box_h if by < ay else by
        _add_line(slide, ax, ay, bx + box_w / 2, conn_y, ann_color, width=1.0, dash=MSO_LINE_DASH_STYLE.DASH)


def build_chart() -> GanttChart:
    scale = TimeScale(
        control_points=[
            (date(2026, 9, 1), 0.00),
            (date(2027, 10, 1), 0.15),
            (date(2028, 8, 1), 0.29),
            (date(2029, 12, 1), 0.47),
            (date(2031, 2, 1), 0.60),
            (date(2031, 6, 1), 0.68),
            (date(2031, 10, 1), 0.76),
            (date(2034, 11, 1), 1.00),
        ]
    )
    ticks = [
        (date(2026, 9, 1), "26/9", "現在", "text_primary"),
        (date(2027, 10, 1), "27/10", "Loom 3", "conditional"),
        (date(2028, 8, 1), "28/8", "Loom 4(予)", "conditional"),
        (date(2029, 12, 1), "29/12", "Board A・Store X", "danger"),
        (date(2031, 2, 1), "31/2", "Board B(予)", "danger"),
        (date(2031, 6, 1), "31/6", "Loom 5(予)", "conditional"),
        (date(2031, 10, 1), "31/10", "OS 22", "text_primary"),
        (date(2034, 11, 1), "34/11", "OS 25", "support"),
    ]
    milestones = [
        (date(2028, 5, 1), "28/5 ライセンス更新"),
        (date(2031, 5, 1), "31/5 ライセンス更新"),
    ]

    support, cond, none = "support", "conditional", "none"

    groups = [
        GanttGroup(
            "第1世代\nBoard A\n120台\nStore X",
            risk_band=(date(2029, 12, 1), date(2031, 2, 1)),
            rows=[
                GanttRow(
                    "Loom 2", sub_group="固定版",
                    segments=[GanttSegment(date(2026, 9, 1), date(2029, 12, 1), none, "内部利用のみ、外部保守")],
                ),
                GanttRow(
                    "Loom 3", sub_group="固定版",
                    segments=[
                        GanttSegment(date(2026, 9, 1), date(2028, 8, 1), support, "内部+外部利用"),
                        GanttSegment(date(2028, 8, 1), date(2029, 12, 1), cond, "延長要確認"),
                    ],
                ),
                GanttRow(
                    "Loom 3/4/5", sub_group="定額版",
                    note=(date(2027, 10, 1), date(2029, 12, 1), "Store XはLoom 4非対応のため移行先になり得ない"),
                ),
            ],
        ),
        GanttGroup(
            "第2世代\nBoard B\n240台\nStore Y",
            rows=[
                GanttRow(
                    "Loom 2/3", sub_group="固定版",
                    segments=[GanttSegment(date(2026, 9, 1), date(2031, 6, 1), none, "内部利用のみ、外部保守")],
                ),
                GanttRow(
                    "(Loom 3)", sub_group="固定版",
                    segments=[GanttSegment(date(2026, 9, 1), date(2029, 12, 1), support, "内部+外部利用")],
                ),
                GanttRow(
                    "Loom 3/4/5", sub_group="定額版",
                    segments=[
                        GanttSegment(date(2026, 9, 1), date(2028, 8, 1), support, "サポート対象"),
                        GanttSegment(date(2028, 8, 1), date(2029, 12, 1), cond, "延長要確認"),
                        GanttSegment(date(2029, 12, 1), date(2031, 6, 1), cond, "ベンダーのサポート可否は要協議"),
                    ],
                ),
                GanttRow(
                    "Weft OS 22",
                    segments=[GanttSegment(date(2026, 9, 1), date(2034, 11, 1), support, "", open_end=True)],
                ),
            ],
        ),
        GanttGroup(
            "第2世代\nBoard C\n80台\nStore Y",
            rows=[
                GanttRow(
                    "Loom 3→4→5", sub_group="定額版",
                    segments=[
                        GanttSegment(date(2026, 9, 1), date(2027, 10, 1), cond, "Loom 3"),
                        GanttSegment(date(2027, 10, 1), date(2028, 8, 1), cond, "Loom 4"),
                        GanttSegment(date(2028, 8, 1), date(2034, 11, 1), support, "Loom 5", open_end=True),
                    ],
                ),
                GanttRow(
                    "Weft OS 22",
                    segments=[GanttSegment(date(2027, 10, 1), date(2034, 11, 1), support, "", open_end=True)],
                ),
            ],
        ),
    ]

    annotations = [
        Annotation(
            date(2027, 10, 1), 0.13,
            "外部向け第2世代はセキュリティ要件で\nLoom 4化が必須(デモ)", "conditional", direction="up",
        ),
        Annotation(
            date(2029, 12, 1), 0.0,
            "ここまでに全利用者を\nBoard B/Cへ移行予定(デモ)", "danger", direction="down",
            emphasis=True,
        ),
        Annotation(
            date(2031, 2, 1), 0.55,
            "保守不可ならBoard C移行が要るが、\n増設/移行の期間が不足(デモ)", "conditional", direction="up",
        ),
        Annotation(
            date(2031, 6, 1), 0.55,
            "ベンダー:27/6までLoom対応\n開発元:27/8にLoom 5公開予定(デモ)", "conditional", direction="down",
        ),
    ]

    return GanttChart(
        scale=scale,
        ticks=ticks,
        groups=groups,
        legend=[
            (support, "ランタイム サポート有"),
            (none, "ランタイム サポート無"),
            ("risk", "ハードウェア サポート無"),
        ],
        milestones=milestones,
        annotations=annotations,
        break_before=date(2034, 11, 1),
        extension_note=(date(2031, 2, 1), 0.27, "Board B保守延長予定(デモ)", "danger"),
    )


def build_slide_spec(theme: dict, subtitle: str) -> Spec:
    header = col(
        [
            text(
                "ボード世代ごとのランタイム選択肢と利用可能期間(架空のデモ)",
                font_size=TYPE["title"] - 6, color=theme["text_primary"], bold=True, height=30,
            ),
            text(
                subtitle,
                font_size=TYPE["caption"] - 1, color=theme["text_muted"], height=14,
            ),
        ],
        gap=SPACE["xs"],
    )

    chart = build_chart()
    chart_canvas = canvas(
        lambda slide, box: render_gantt(slide, box, chart, theme),
        grow=1, basis=0, background=theme["card"], radius=12,
    )

    footer = row(
        [
            text("Boxloom デモ", font_size=TYPE["caption"], color=theme["text_muted"]),
            text("v11", font_size=TYPE["caption"], color=theme["text_muted"], align=PP_ALIGN.RIGHT),
        ],
        justify_content=JustifyContent.SPACE_BETWEEN,
    )

    return col(
        [header, chart_canvas, footer],
        gap=SPACE["sm"],
        padding=SPACE["lg"],
        width=SLIDE_W_PT,
        height=SLIDE_H_PT,
        background=theme["bg"],
    )


def main(out_path: str, theme: dict, subtitle: str) -> float:
    spec = build_slide_spec(theme, subtitle)
    _compute_layout(spec, 0.0, 0.0, SLIDE_W_PT, SLIDE_H_PT)

    ratio = content_fill_ratio(spec)
    warning = lint_fill_ratio(ratio)
    print(f"[{out_path}] content fill ratio: {ratio:.1%}" + (f"  {warning}" if warning else "  OK"))

    prs = Presentation()
    prs.slide_width = pt_to_emu(SLIDE_W_PT)
    prs.slide_height = pt_to_emu(SLIDE_H_PT)
    slide = prs.slides.add_slide(prs.slide_layouts[6])

    emit(spec, slide)
    emit_fill_ratio_notes(slide, warning)

    prs.save(out_path)
    print(f"saved: {out_path}")
    return ratio


if __name__ == "__main__":
    main(
        "poc_output_v11_dark.pptx", THEME_DARK,
        "v11: Taffy依存を排し、自作の箱配置エンジン(box_engine.py)だけで描画",
    )
    main(
        "poc_output_v11_light.pptx", THEME_LIGHT,
        "v11: 自作エンジン + ブルー基調の明るいテーマ(内容は一切変更していない)",
    )
