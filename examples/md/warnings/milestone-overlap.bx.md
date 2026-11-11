---
title: Gantt の節目が近すぎる例
author: Boxloom デモ
template: light
cover: false
---

# 監査・公開・契約の更新が、同じ月に重なる

```gantt
span: 2027-01 .. 2027-12
milestones:
  - {at: 2027-06, label: 外部監査}
  - {at: 2027-06, label: 一般公開}
  - {at: 2027-07, label: 契約の更新}
  - {at: 2027-11, label: 年次の見直し}
legend:
  support: 予定どおり
  conditional: 要調整
groups:
  - id: build
    label: [開発]
    rows:
      - label: 中核
        segments:
          - {from: 2027-01, to: 2027-06, status: support, label: 実装}
          - {from: 2027-06, to: 2027-12, status: conditional, label: 改善}
      - label: 画面
        segments:
          - {from: 2027-02, to: 2027-08, status: support, label: 仕上げ}
  - id: ops
    label: [運用]
    rows:
      - label: 移行
        segments:
          - {from: 2027-06, to: 2027-12, status: support, label: 段階的な移行, openEnd: true}
```

- 2027 年 6 月の2つの節目は段をずらして置けるが、7 月の「契約の更新」は2段のどちらにも空きが無く、重なる
- 直すには、近い節目を1つにまとめる・どれかを注釈(annotations)にする・時間軸を広げる(span を短くする)
