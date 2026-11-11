# Contributing to Boxloom

Thank you for considering a contribution. Boxloom has two kinds of work with different
rules. **Code is open to everyone. Visual decisions belong to a small group.**

## 1. Who decides what

| Area | Examples | How it is decided |
|---|---|---|
| **Engine and tooling** | layout engine, OOXML writer, parser, `check`, CLI, library API, CI, font metrics, bug fixes, performance, tests | Normal review by any maintainer. Contributions are welcome broadly. |
| **Taste** | adding, removing or changing a **pattern** (a slide type, a body block, a figure subsystem); a template's **values** (colors, type scale, spacing, fonts); how a pattern **looks** (layout of its parts, emphasis, markers) | Decided by the **taste owners**, a small group of maintainers responsible for the visual direction. Open an issue first; a PR without prior agreement may be declined even if the code is good. |

Why the split: whether a pattern belongs in the vocabulary, and how it should look, is a
design question rather than an engineering one. A consistent result needs a consistent
eye, the way projects like Tailwind CSS or shadcn/ui are steered by a few people. The
reasoning is in [`docs/vision.md`](docs/vision.md) §6.

Until the project names its taste owners publicly, the repository owner is the only one.

## 2. Principles every change must keep

These are not up for debate in a single PR (open an issue to discuss them):

- **Meaning and looks stay separate.** The input (Markdown and IR) carries meaning only.
  No colors, sizes, positions or fonts in the input. The only exception is the `shapes`
  canvas, which is limited to 0–1 coordinates and theme role names.
- **The IR is designed from meaning.** Conveniences of the Markdown syntax must not leak
  into the IR.
- **The vocabulary is closed and strict.** Unknown blocks, keys and attributes are errors,
  not ignored.
- **The output is native PPTX.** It uses real text, placeholders, tables, connectors and
  charts. It is never an image of a slide.
- **Never shrink or move content silently.** Report overflow with `layout-overflow`
  instead.
- **Zero dependencies.** The Go module uses only the standard library. Python tools under
  `tools/` are for verification only.
- **One error format.** Errors and warnings use `{file, line?, column?, path, code,
  message, expected?, hint?}`.

## 3. Proposing a new pattern

New patterns go through the taste owners. Open an issue that contains:

1. **The content structure** it serves, stated without reference to looks (for example,
   "steps without dates, where each step's output feeds the next").
2. **Where it sits in the selection rules** of [`docs/patterns.md`](docs/patterns.md) §4.
   Give its position among R1–R14, one good example and one bad example, and explain why
   the existing patterns or `shapes` are not enough.
   - Repeated use of `shapes` for the same shape is the strongest evidence.
3. **Block or subsystem**, using the test in `docs/patterns.md` §2: a subsystem if the
   positions of elements carry meaning, a block if position is only reading order.
4. **The IR shape**, written from meaning only.

Once agreed, a pattern PR must update all of these together:

- `docs/patterns.md`: the table in §3 and the rules in §4.
- `docs/content-syntax.md`: the Markdown syntax.
- `docs/ir.md` and `schema/`: the IR. Run `go generate` to refresh `boxloom/schemas/`.
- The template parts and their declarations, so that `boxloom schema` lists the pattern.
- `skills/boxloom/SKILL.md`.
- Examples under `examples/md` and `examples/ir`, plus tests.

## 4. Development

```sh
export PATH=/usr/local/go/bin:$PATH   # Go 1.24+
cd boxloom
go vet ./... && go test ./...
go run ./cmd/boxloom check ../examples/md/demo-deck.bx.md
```

Before sending a PR, run the CI script from the repository root:

```sh
sh tools/ci.sh go python pptx      # add `readers` if LibreOffice is installed
```

- The `pptx` stage validates the generated files against the ECMA-376 XSDs. It fetches
  them with `tools/fetch_ooxml_schemas.py`; they are not committed.
- If your change alters generated PPTX in `boxloom/out/`, say so in the PR and explain
  why. Include before/after screenshots for any visual change.
- Use fictional data in examples and tests. Do not include names of real companies,
  products, teams or people, or anything taken from private documents.

## 5. Commits and PRs

- By contributing, you agree that your contribution is licensed under the
  [Apache License, Version 2.0](LICENSE), the license of this project (Section 5 of the
  license). No separate CLA is required.

- Keep commits focused. Write the message in English and explain *why*.
- One topic per PR. Update the documentation in the same PR as the code.
- Design documents under `docs/` are currently written in Japanese. You may write new
  sections in English; a maintainer will help keep them consistent.
