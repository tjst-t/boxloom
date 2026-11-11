---
title: ランタイム移行計画(Gantt の全体)
author: Boxloom デモ
template: dark
cover: false
---

# ボード世代ごとのランタイム選択肢と利用可能期間(架空のデモ)

```gantt
span: 2026-09 .. 2034-11
axisBreakBefore: 2034-11
ticks:
  - {at: 2026-09, label: 現在}
  - {at: 2027-10, label: Loom 3, status: conditional}
  - {at: 2028-08, label: Loom 4(予), status: conditional}
  - {at: 2029-12, label: Board A・Store X, status: danger}
  - {at: 2031-02, label: Board B(予), status: danger}
  - {at: 2031-06, label: Loom 5(予), status: conditional}
  - {at: 2031-10, label: OS 22}
  - {at: 2034-11, label: OS 25, status: support}
milestones:
  - {at: 2028-05, label: ライセンス更新}
  - {at: 2031-05, label: ライセンス更新}
legend:
  support: ランタイム サポート有
  none: ランタイム サポート無
  risk: ハードウェア サポート無
groups:
  - id: gen1
    label: [第1世代, Board A, 120台, Store X]
    risk: 2029-12 .. 2031-02
    rows:
      - label: Loom 2
        sub: 固定版
        segments:
          - {from: 2026-09, to: 2029-12, status: none, label: 内部利用のみ、外部保守}
      - label: Loom 3
        sub: 固定版
        segments:
          - {from: 2026-09, to: 2028-08, status: support, label: 内部+外部利用}
          - {from: 2028-08, to: 2029-12, status: conditional, label: 延長要確認}
      - label: Loom 3/4/5
        sub: 定額版
        note: {from: 2027-10, to: 2029-12, text: Store XはLoom 4非対応のため移行先になり得ない}

  - id: gen2b
    label: [第2世代, Board B, 240台, Store Y]
    rows:
      - label: Loom 2/3
        sub: 固定版
        segments:
          - {from: 2026-09, to: 2031-06, status: none, label: 内部利用のみ、外部保守}
          - {from: 2031-06, to: 2034-11, status: danger, label: Board B保守延長予定(デモ), tentative: true}
      - label: (Loom 3)
        sub: 固定版
        segments:
          - {from: 2026-09, to: 2029-12, status: support, label: 内部+外部利用}
      - id: gen2b-flat
        label: Loom 3/4/5
        sub: 定額版
        segments:
          - {from: 2026-09, to: 2028-08, status: support, label: サポート対象}
          - {from: 2028-08, to: 2029-12, status: conditional, label: 延長要確認}
          - {from: 2029-12, to: 2031-06, status: conditional, label: ベンダーのサポート可否は要協議}
      - label: Weft OS 22
        segments:
          - {from: 2026-09, to: 2034-11, status: support, openEnd: true}

  - id: gen2c
    label: [第2世代, Board C, 80台, Store Y]
    rows:
      - label: Loom 3→4→5
        sub: 定額版
        segments:
          - {from: 2026-09, to: 2027-10, status: conditional, label: Loom 3}
          - {from: 2027-10, to: 2028-08, status: conditional, label: Loom 4}
          - {from: 2028-08, to: 2034-11, status: support, label: Loom 5, openEnd: true}
      - label: Weft OS 22
        segments:
          - {from: 2027-10, to: 2034-11, status: support, openEnd: true}

annotations:
  - at: 2027-10
    target: gen1
    status: conditional
    text: [外部向け第2世代はセキュリティ要件で, Loom 4化が必須(デモ)]
  - at: 2029-12
    target: gen1
    status: danger
    emphasis: true
    text: [ここまでに全利用者を, Board B/Cへ移行予定(デモ)]
  - at: 2031-02
    target: gen2b
    status: conditional
    text: [保守不可ならBoard C移行が要るが、, 増設/移行の期間が不足(デモ)]
  - at: 2031-06
    target: gen2b-flat
    status: conditional
    text: [ベンダー:27/6までLoom対応, 開発元:27/8にLoom 5公開予定(デモ)]
```
