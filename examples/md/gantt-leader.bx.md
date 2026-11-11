---
title: 注釈の引き出し線と軸の文字(Gantt)
author: Boxloom デモ
template: light
cover: false
---

# 注釈の引き出し線は、目盛りと節目の文字を横切らない

```gantt
span: 2027-01 .. 2028-12
ticks:
  - {at: 2027-01, label: 開始}
  - {at: 2027-06, label: 目標期限, status: danger}
  - {at: 2027-09, label: 中間審査, status: conditional}
  - {at: 2028-06, label: 本番, status: support}
milestones:
  - {at: 2027-06, label: 仕様の凍結}
  - {at: 2028-02, label: 受け入れ試験}
legend:
  support: 予定どおり
  conditional: 要調整
  danger: 遅れの恐れ
groups:
  - id: build
    label: [開発]
    rows:
      - id: core
        label: 中核
        segments:
          - {from: 2027-01, to: 2027-06, status: danger, label: 設計}
          - {from: 2027-06, to: 2028-06, status: support, label: 実装と試験}
      - label: 画面
        segments:
          - {from: 2027-03, to: 2027-09, status: conditional, label: 試作}
          - {from: 2027-09, to: 2028-06, status: support, label: 仕上げ}
  - id: ops
    label: [運用]
    rows:
      - label: 移行
        segments:
          - {from: 2028-02, to: 2028-12, status: support, label: 段階的な移行, openEnd: true}
annotations:
  - at: 2027-06
    target: core
    status: danger
    text: [目標期限と仕様の凍結が同じ月(架空)]
  - at: 2027-09
    target: core
    status: conditional
    text: [中間審査で画面の方針を決める(架空)]
  - at: 2028-02
    target: build
    status: support
    text: [受け入れ試験の結果で移行を始める(架空)]
```
