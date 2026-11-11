---
title: コネクタの角度
subtitle: 斜めに並んだ図形を繋ぐ例(架空)
author: Boxloom デモ
template: light
cover: false
---

# 斜めに並んだ図形を繋ぐ

```shapes
aspect: 2.6
items:
  - {id: s1, kind: rect, x: 0.0, y: 0.05, w: 0.25, h: 0.2, text: 受付, line: grid}
  - {id: t1, kind: rect, x: 0.3, y: 0.32, w: 0.25, h: 0.2, text: 振り分け, line: grid}
  - {id: s2, kind: rect, x: 0.0, y: 0.6, w: 0.2, h: 0.18, text: 問い合わせ, line: grid}
  - {id: t2, kind: roundRect, x: 0.28, y: 0.8, w: 0.2, h: 0.18, text: 回答, line: grid}
  - {id: s3, kind: rect, x: 0.62, y: 0.1, w: 0.14, h: 0.3, text: 集計, line: grid}
  - {id: t3, kind: ellipse, x: 0.8, y: 0.55, w: 0.18, h: 0.3, text: 報告, line: grid}
  - {kind: connector, from: s1, to: t1, arrow: true}
  - {kind: connector, from: s2, to: t2, arrow: true}
  - {kind: connector, from: s3, to: t3, arrow: true}
```
