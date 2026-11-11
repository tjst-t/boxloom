---
name: boxloom
description: Write slide decks as Boxloom Markdown (.bx.md) and turn them into native, editable PPTX. Use when asked to make a presentation / PowerPoint / slides with Boxloom, or when editing a .bx.md file. Covers the syntax, how to pick a pattern for each piece of content, density limits measured by the layout engine, and the check → fix loop.
---

# Boxloom: writing decks

Boxloom separates **meaning** from **looks**. You write only what the content *is*
(a list of points, a comparison of options, a dated plan). The template decides every
color, font size, margin and position. You never write a color, a size or a coordinate
outside the one escape hatch (`shapes`, last resort).

The loop is always:

1. `boxloom schema` — read what the current templates accept (do this first).
2. Write `deck.bx.md`.
3. `boxloom check -format json deck.bx.md` — fix every error, then every warning you can.
4. Repeat 3 until `"ok": true` and no `layout-overflow` warning.
5. `boxloom build -o deck.pptx deck.bx.md` (flags before the file).

Commands run from the `boxloom/` directory of the repository:
`go run ./cmd/boxloom <schema|check|build|parse> …` (Go 1.24+, no other dependency).

## 1. Read the schema first

`boxloom schema [-template light|dark]` prints one JSON document. It is the source of
truth; if this file and the schema disagree, **the schema wins**.

- `markdown.rules` — the syntax rules (short list; obey all of them).
- `markdown.frontMatter` — keys allowed in the `---` header (`title` is required).
- `templates[].blocks` — the fenced blocks (`:::name`) this template knows, with
  `inside` (where it may appear), `contains` (what goes in it) and `minChildren`.
- `templates[].subsystems` — the figure types (```` ```name ```` code blocks), with
  `markdown` (their extra rules) and `schema` (a JSON Pointer into `irSchema`
  giving every allowed key).
- Anything not listed is an error. Do not invent block names, attributes or keys.
- Templates are chosen by name (`template: light` in the front matter, or
  `-template dark`). A custom template is a value table file passed by path:
  `-template ./my.json` (works for `schema`, `check`, `build`, `fonts`; example
  `examples/templates/forest.json`). When the deck will be built with a custom
  template, run `schema` and `check` with the same `-template` — role names and
  text styles come from that file.
- `variant` (`# Title {variant=…}`) exists in the syntax, but the current templates
  declare no variants (`templates[].variants` is empty), so do not use it.

The output is large. To see just what you need:

```sh
go run ./cmd/boxloom schema -template light > /tmp/schema.json
python3 -c "import json;d=json.load(open('/tmp/schema.json'));print(json.dumps({'rules':d['markdown']['rules'],'blocks':d['templates'][0]['blocks'],'subsystems':[(x['name'],x['markdown']) for x in d['templates'][0]['subsystems']]},ensure_ascii=False,indent=1))"
python3 -c "import json;d=json.load(open('/tmp/schema.json'));print(json.dumps(d['irSchema']['\$defs']['gantt'],ensure_ascii=False,indent=1))"   # all gantt keys
```

## 2. Syntax in one screen

```markdown
---
title: Moving the reporting service to the new cluster
subtitle: Plan and options (fictional example)
author: Platform team
template: light
---

# The old cluster loses vendor support in March 2028

- The vendor ends security fixes on **2028-03-31**
- 14 services still run there; 9 can move without code changes
- Moving takes about 6 weeks per service group

:::note
All dates are illustrative
:::

:::notes
Speaker notes go here. Plain paragraphs and bullets only.
:::
```

- `# Title` starts a slide. **Write the title as the conclusion**, not the topic
  ("The old cluster loses support in 2028", not "Cluster status").
- `---` is not a slide separator (it is an error). `##` and deeper headings are errors
  (a group title is written as `:::column Title`, not as a subheading).
- Body: paragraphs, `- ` / `1. ` bullets (two levels max), `**emphasis**`
  (one kind of emphasis only), `` `code` ``, `[text](url)`, trailing `\` for a
  meaningful line break. No italics, strikethrough, footnotes, HTML.
- Fenced blocks: open with `:::name` and close with the **same number of colons**.
  When nesting, the outer fence gets **one more colon** than the inner one:

  ```markdown
  :::::compare
  ::::option Keep the old cluster
  :::pros
  - No migration work this year
  :::
  :::cons
  - No security fixes after March 2028
  :::
  ::::
  ::::option Move to the new cluster
  :::pros
  - Supported until 2033
  :::
  :::cons
  - Six weeks of work per service group
  :::
  ::::
  :::::
  ```

- `:::notes` (speaker notes) must sit at the top level of a slide, once per slide.
- Figures are fenced code blocks whose language is the subsystem name, body in a
  restricted YAML: no anchors/aliases (`&`, `*`), no tags, no multi-line values,
  no `|`/`>` blocks; booleans only `true`/`false`; dates are strings
  `YYYY-MM` or `YYYY-MM-DD`; ranges `2027-01 .. 2027-06`.
  **Quote any value that starts with `*`, `&`, `!` or contains `": "`**, e.g.
  `text: ["**Slowest step**", 5 days on average]` — an unquoted `**…**` at the start of
  a flow item is read as a YAML alias and rejected.

## 3. Choosing a pattern for each piece of content

Decide the *type of content* before writing. Apply these rules **in order** and use the
first one that matches. (Full rule set with good/bad examples: `docs/patterns.md` §4.)

Available now (check `boxloom schema` — more may have been added):

| Content | Use | Notes |
|---|---|---|
| A chapter break | `# Part 2: How we migrate {type=section}` | Heading text is the chapter title; nothing but `:::notes` may follow. Not a table of contents |
| The slide says exactly one sentence; the evidence is on other slides | `# Migration must move two years earlier {type=statement}` | Heading text is the sentence; nothing but `:::notes` may follow. If you would add reasons, use a normal slide with the conclusion as title |
| 1–4 numbers to show strongly, with no series to compare | `::::stat` → `:::value 3 → 12 {unit=models}` + one-line label inside | 5+ numbers or a time series is a chart/table, not stat. The value is written as-is and must contain a digit |
| Someone else's words | `:::quote Source text` + the words inside | The source (heading text) is required. Never quote your own conclusion |
| One sentence to emphasise inside a normal slide | `:::takeaway` + one sentence | At most one per slide, usually last |
| Rows × columns of plain text or numbers | GFM table: `\| A \| B \|` + `\| --- \| --- \|` + rows | First row is the header; every row has one cell per column (`table-shape`). **No alignment markers** (`:--`, `--:` are errors); numeric columns are right-aligned for you. Cells are inline only. A line `: caption` right after the table is its caption. Ratings in cells → `matrix` (below) |
| Rows × columns where every cell is a rating (supported / conditional / not / risk) | ```` ```matrix ```` | `columns: [A, B]`, `rows: [{label, cells: [{status, text?}]}]`, optional `legend: {support: Good}`. One cell per column (`matrix-cells`). Cells that are plain text or numbers are a table, not a matrix |
| 3–7 steps without dates, in a direction (the output of one is the input of the next) | ```` ```process ```` | `steps: [{title, detail?, status?, emphasis?}]`, optional `caption`. Numbers (1. 2. …) and arrows are added for you; long processes wrap to a second row |
| Two or more options, each with pros and cons | `:::compare` → `:::option` → `:::pros` / `:::cons` | If 3+ shared criteria apply to every option, a matrix fits better (above) |
| A parent with children (and grandchildren): team set-up, ownership | ```` ```orgchart ```` | `root: {title, detail?}`, `units: [{title, detail?, members: [{title, detail?}]}]`. 3 levels max. Not for unrelated teams side by side |
| Stages whose count shrinks (visits → sign-ups → paid) | ```` ```funnel ```` | `stages: [{label, value, note?}]`, optional `unit`. `value` is a number or text; bar widths are stepped, not proportional. Values that grow are warned (`funnel-increase`) — use `process` for plain steps |
| A dated plan, periods, deadlines, statuses over time | ```` ```gantt ```` | States are `support`, `conditional`, `none`, `danger` (roles, not colors). Hardware/support-risk periods are a group `risk: A .. B`. Annotations point at a group/row `id` via `target`; the subsystem places them |
| 2–3 parallel groups read side by side ("this year / next year / not doing") | `::::columns` → `:::column Group title` (2 or 3) | Each group holds paragraphs, bullets, one table, one image or `:::note`. No nested columns, compare, gallery or figures. Widths are not numbers: mark the one main group `{role=main}` (the others support it) or leave all equal. 4+ one-line groups → bullets. Not for one argument split in two |
| An image that shows the point (screenshot, photo, drawing) | a paragraph containing only `![alt text](images/x.png "caption")` | Alt text is required (`image-alt`). Path is relative to the deck file; no URLs, absolute paths or `..` (`image-path`). PNG or JPEG only (`image-format`). Kept in proportion, never cropped. Image + text side by side → `::::columns` with the image in one group |
| 2–6 images compared or in sequence | one paragraph with 2–6 `![alt](path "caption")` | Becomes a gallery (one row for 2–3, a grid for 4–6). 7+ is an error: split the slide |
| Parallel points | bullets | 2 levels max |
| Connected prose | paragraph | 2–3 sentences |
| A caveat / footnote-like remark | `:::note` | One short line |
| A diagram whose positions/links carry meaning and none of the above fits | ```` ```shapes ```` | **Last resort**, see §3.1 |

In all four figures above you write only text, `status` (`support`, `conditional`,
`none`, `danger`) and `emphasis: true`; position, size and colour come from the template.

Patterns described in `docs/patterns.md` but **not in the schema yet** (chart) are
errors until `boxloom schema` lists them. When the schema lists them, prefer them in the
order of `docs/patterns.md` §4 (R1–R14).

Until then:

- A short number series → a `table` (or bullets with the numbers), or a `gantt` if the
  values are dated periods. Do not draw charts with `shapes`.

Example of the text patterns (full deck: `examples/md/text-patterns.bx.md`):

```
# Support tickets fell by 40% after the redesign

::::stat
:::value 3 → 12 {unit=models}
Device models supported
:::
:::value −40 {unit=%}
Tickets per month (year on year)
:::
::::

:::quote Front-desk staff A, interview in Sept 2026
It now ends on one screen.
:::

:::takeaway
Most of the drop came from the **first registration** flow
:::
```

Example of groups, a table and an image (full deck: `examples/md/blocks.bx.md`):

```
# Merging the front desks halves the handling time

:::::columns
::::column Merge the front desks {role=main}
One screen replaces four. Duplicate input disappears.

- Screens: 4 → 1
::::
::::column Measured
| Item | Now | After |
| --- | --- | --- |
| Handling time | 6 min | 3 min |
::::
:::::

# The new screen fits the list and the totals on one page

![Prototype screen: desks listed on the left, weekly counts as bars on the right](images/screen.png "Prototype (fictional)")
```

`>` blockquotes are an error; use `:::quote`. A unit containing spaces cannot be
written yet (attribute values are single words).

Text next to a figure: put the one or two lines of conclusion as a paragraph or bullets
**after** the figure; use `:::note` only for a caveat ("dates are illustrative").

### 3.1 `gantt` example

```gantt
span: 2027-01 .. 2029-03
ticks:                                   # labelled dates on the axis; status colors the label
  - {at: 2027-01, label: Today}
  - {at: 2028-04, label: Online by default, status: support}
milestones:                              # one-off events shown with a star above the axis
  - {at: 2027-09, label: Budget approval}
legend:                                  # keys: support, conditional, none, danger, risk
  support: On track
  conditional: Needs agreement
  risk: Staff shortage
groups:
  - id: portal-team
    label: [Portal, 4 people]            # a list = several lines
    risk: 2027-06 .. 2027-10             # a period with a property (not a status)
    rows:
      - label: Build
        segments:
          - {from: 2027-01, to: 2027-09, status: support, label: First release}
          - {from: 2027-09, to: 2029-03, status: conditional, label: Extensions, openEnd: true}
      - id: portal-migrate
        label: Migration
        segments:
          - {from: 2027-10, to: 2028-04, status: danger, label: Paper forms, tentative: true}
annotations:                             # placed automatically next to the target
  - at: 2028-04
    target: portal-migrate               # a group id or a row id
    status: danger
    text: [Paper forms end here, if the budget passes]
```

- Segment `status` is one of `support`, `conditional`, `none`, `danger`. `risk` is not a
  status: it is only a group's `risk: A .. B` range and a `legend` key.
- `openEnd: true` = continues past the chart; `tentative: true` = not yet decided
  (drawn dashed). A row may have `note: {from, to, text}` instead of segments.
- Use `ticks` for dates readers should orient by; `milestones` for events.
- Ids are unique across the whole deck.
- Segment labels wrap inside the bar (up to 3 lines; the row grows). If a bar is too
  narrow even for that, the label is not drawn and `check` warns `gantt-segment-label`.
  Keep segment labels to a few words; put longer explanations in an `annotation` or a
  row `note`.

### 3.2 `shapes` (raw shapes, last resort)

Use only when all are true:

1. You can say why each earlier rule does not fit.
2. There are 3+ shapes and their positions or links carry meaning.
3. It is not a shape that has (or will have) a dedicated type: boxes with arrows in a
   row (process), a parent box over child boxes (org chart), a grid of ratings (matrix).

A flow that branches (one box feeding several, or several feeding one) and a system
diagram with layers are typical legitimate uses; a straight row of steps is not.

Rules: coordinates and sizes are fractions 0–1 of the frame (`x+w ≤ 1`, `y+h ≤ 1`);
text size is a `textStyle` step (`small`, `subPoint`, `optionItem`, `body`,
`optionLabel`); `connector` links two shape `id`s with a straight line between side
midpoints. Colors are theme role names only — the complete list:

| Role | Use for |
|---|---|
| `bg`, `card`, `card_alt` | backgrounds; `card_alt` is the default shape fill |
| `text_primary`, `text_muted` | text; muted for captions |
| `grid` | quiet lines and borders |
| `accent` | the one thing to look at; put text on it with `textColor: on_conditional` |
| `support`, `conditional`, `none`, `danger` | status: good / with conditions / not applicable / bad |
| `risk` | a pale fill for risky periods or areas |
| `on_conditional` | text on `accent` or `conditional` fills |

```shapes
aspect: 2.6
items:
  - {id: users, kind: roundRect, x: 0, y: 0.38, w: 0.18, h: 0.24, text: Residents, textStyle: body, line: grid}
  - {id: portal, kind: rect, x: 0.3, y: 0.38, w: 0.18, h: 0.24, text: Portal, textStyle: body, fill: accent, textColor: on_conditional}
  - {id: tax, kind: rect, x: 0.65, y: 0.05, w: 0.2, h: 0.2, text: Tax office, line: grid}
  - {id: welfare, kind: rect, x: 0.65, y: 0.4, w: 0.2, h: 0.2, text: Welfare, line: grid}
  - {id: parks, kind: rect, x: 0.65, y: 0.75, w: 0.2, h: 0.2, text: Parks, line: grid}
  - {kind: connector, from: users, to: portal, arrow: true}
  - {kind: connector, from: portal, to: tax, arrow: true}
  - {kind: connector, from: portal, to: welfare, arrow: true}
  - {kind: connector, from: portal, to: parks, arrow: true}
  - {kind: text, x: 0, y: 0.85, w: 0.5, h: 0.12, text: "One entry point, three back offices", textStyle: small, color: text_muted}
```
 Never mention a color by name in the text
("the red box") — colors change with the template; say "the highlighted box".

## 4. Density: measured limits

The slide body is a fixed box. The engine never shrinks text and never moves content
to the next slide; **you** split slides. Measured on the default templates (16 pt body
text, 16:9):

| Quantity | Limit |
|---|---|
| Body area of a content slide | ~414 pt high ≈ 20 lines of body text |
| One-line bullets on an otherwise empty slide | 15 fit; **aim for 3–6** |
| Characters per full-width line | ~65 CJK characters, ~100 Latin characters |
| Characters per line inside a `compare` column | ~30 CJK characters |
| Table rows (12 pt cells, one line each) | ~16 under a title with nothing else; keep tables to ≤ 8 rows and ≤ 5 columns |
| Gap between blocks | 16 pt (a new block costs this plus its own height) |

Targets that read well (not hard limits):

- One message per slide; the title states it.
- 3–6 bullets per list, each ≤ 1 line (≤ ~40 CJK / ~70 Latin characters is comfortable).
- `compare`: 2–3 options, 1–3 pros and 1–3 cons each, each ≤ 1 column line.
- A slide with a figure (`gantt`, `process`, `orgchart`, `matrix`, `funnel`, `shapes`) gets at most 1–2 short lines of text
  besides the figure.
- `fill-ratio` warns when less than 65% of the body is used. A slide with only text
  needs about 13 lines to pass, so **short text slides (3–6 bullets, one `compare`)
  normally get this warning — that is expected.** Treat it as a question, not an
  error: if two neighbouring slides are both thin and make the same point, merge them;
  if a figure would carry the point, add it; otherwise leave the warning. Never pad a
  slide with filler to pass it.

## 5. The check → fix loop

```sh
go run ./cmd/boxloom check -format json deck.bx.md
```

returns

```json
{"ok": false,
 "errors":   [{"file": "deck.bx.md", "line": 14, "column": 1, "path": "/slides/1/body/0",
               "code": "unknown-block", "message": "…", "expected": ["compare", "note"], "hint": "…"}],
 "warnings": [{"file": "deck.bx.md", "line": 38, "path": "/slides/2/body/2",
               "code": "layout-overflow", "message": "block (points) overflows the body area by 75.1pt …"}],
 "capacity": [{"path": "/slides/2", "bodyArea": 414.1, "need": 518.7, "remaining": -104.6,
               "blockGap": 16, "bodyLineHeight": 18.9, "remainingBodyLines": 0,
               "blocks": [{"path": "/slides/2/body/2", "line": 38, "type": "points",
                           "need": 180.3, "lines": 7, "overflow": 75.1}]}]}
```

- `line`/`column` point into your Markdown; `path` points into the IR
  (`/slides/S/body/N` = slide S counting the cover as 0, block N). `build` prints the
  same warnings with 1-based slide numbers (`/slides/1` is "slide 2" when there is a
  cover) and still exits 0; only errors stop `build`.
- Branch on `code`. Use `expected` and `hint` when present.
- Exit code: 1 if errors, 0 if only warnings.

Fixes by code:

| code | Fix |
|---|---|
| `unclosed-block` | Count colons: each `:::name` needs a closing line with the same number of colons; outer fences have one more colon than inner ones |
| `misplaced-content` | The block is in the wrong place or has too few children (`option` only inside `compare`, `compare` needs 2+ options, `notes` only at top level) |
| `unknown-block`, `unknown-attribute`, `unknown-key` | Use only names from `expected` / `boxloom schema` |
| `planned-syntax`, `unsupported-syntax`, `raw-html` | Rewrite with what exists (§3); no `##`, italics, table alignment markers or HTML |
| `table-shape` | Give every table row exactly as many cells as the header (empty cell: `\| \|`) |
| `image-alt`, `image-path`, `image-missing`, `image-format` | Write alt text; use a path relative to the deck file (no URL, `/…` or `..`); make sure the file exists next to the deck and is PNG or JPEG |
| `yaml` | Restricted YAML (§2): quote values starting with `*`, remove anchors and multi-line values |
| `missing-key`, `invalid-value`, `schema` | Compare with the subsystem schema in `irSchema` |
| `date-order`, `date-valid`, `within-span` | Gantt dates: `from` before `to`, real dates, inside `span` |
| `target-exists`, `unique-id` | `target`/connector ends must name an existing `id`; ids are unique across the whole deck |
| `template-values` | The custom template file (`-template ./my.json`) is invalid: the `file`/`line`/`path` point into that file, not your deck. Fix the template or use a bundled one |
| `shape-bounds`, `role-exists`, `connector-self` | `shapes`: keep within 0–1, use theme role names, connect two different shapes |
| `matrix-cells` | `matrix`: give every row exactly one cell per column |
| `layout-overflow` (warning) | **Split the slide** (below). Do not just delete content silently. Inside `columns` the path ends in `/columns/C/body/K`: shorten that group or move it to its own slide |
| `fill-ratio` (warning) | Slide is mostly empty: merge with a neighbour, add the missing figure or evidence, or keep it if it is a deliberate one-line message |
| `gantt-segment-label` (warning) | A Gantt bar is too narrow for its label: shorten the label, or move the explanation to an `annotation` / row `note` and leave the segment label short or empty |
| `shapes-overlap`, `shapes-text-overflow`, `annotation-overlap`, `subsystem-min` (warnings) | Move/enlarge shapes, shorten text, or give the figure its own slide |
| `figure-text-overflow` (warning) | A `process`/`orgchart`/`matrix`/`funnel` box cannot hold its text (an unbreakable word is too wide, or the figure got too little height): shorten the text or give the figure its own slide |
| `funnel-increase` (warning) | A funnel value grows: check the numbers, or use `process` if the stages do not shrink |

### Splitting an overflowing slide

1. In `capacity`, find the slide with `remaining < 0`.
2. Find the first block with `overflow > 0`. Everything from that block on moves to a
   new slide placed right after it.
3. Give the new slide its own conclusion title (not "… (cont.)").
4. If one block alone is taller than `bodyArea`, split the block itself (a long list
   into two lists by theme, a compare with 4 options into two compares).
5. Run `check` again.

Before adding content to a slide that already passes, use `capacity`: a new bullet of
one line fits if `remainingBodyLines ≥ 1`; a new block fits if its height is below
`remaining − blockGap`.

## 6. Hard rules

- Only write meaning: no colors, sizes, positions, fonts, alignment (except inside
  `shapes`, with role names and 0–1 coordinates).
- Do not use names of real companies, products or people in examples unless the user
  supplied them as the content.
- Every slide title is a conclusion. One message per slide.
- Finish only when `check` reports no errors and no `layout-overflow`.
