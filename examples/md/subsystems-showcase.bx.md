---
title: 新サービス「Kumo」立ち上げ計画
subtitle: サブシステム(Gantt・図形)をふんだんに使ったデモ(すべて架空)
author: Boxloom デモ
template: light
---

:::notes
このデコはすべて架空の会社・架空のサービスの話です。Boxloom のサブシステムの見本として使います。
:::

# 論点

- 架空の会社 Hoshizora が、データを外に出さない **セルフホスト型** の集計サービス Kumo を立ち上げる
- 2027年4月に限定公開、**2027年10月** に一般公開を目指す
- 一般公開の前に、認証まわりの外部監査を通す必要がある

:::note
数字・日付・組織はすべて架空のデモ
:::

# システム構成

要求は **Web 層** から API 層に入り、認証・データベース・集計ジョブへ分かれる。

```shapes
aspect: 2.6
items:
  - {kind: text, x: 0, y: 0, w: 0.4, h: 0.1, text: 利用者の環境の中で動く, color: text_muted}
  - {kind: text, x: 0.7, y: 0, w: 0.3, h: 0.1, text: 内部のサービス, color: text_muted}
  - {kind: line, from: [0.665, 0.02], to: [0.665, 0.98], color: grid}
  - {id: arch-client, kind: roundRect, x: 0, y: 0.38, w: 0.15, h: 0.24, text: 利用者の端末, textStyle: body, line: grid}
  - {id: arch-web, kind: roundRect, x: 0.22, y: 0.38, w: 0.15, h: 0.24, text: Web 層, textStyle: body, line: accent}
  - {id: arch-api, kind: rect, x: 0.44, y: 0.38, w: 0.15, h: 0.24, text: API 層, textStyle: body, fill: accent, textColor: on_conditional}
  - {id: arch-auth, kind: rect, x: 0.72, y: 0.12, w: 0.18, h: 0.2, text: 認証サービス, line: danger}
  - {id: arch-db, kind: ellipse, x: 0.72, y: 0.4, w: 0.18, h: 0.2, text: データベース, line: grid}
  - {id: arch-jobs, kind: rect, x: 0.72, y: 0.68, w: 0.18, h: 0.2, text: [集計ジョブ, (夜間に1回)], line: grid}
  - {kind: connector, from: arch-client, to: arch-web, arrow: true}
  - {kind: connector, from: arch-web, to: arch-api, arrow: true, color: accent}
  - {kind: connector, from: arch-api, to: arch-auth, arrow: true, color: danger}
  - {kind: connector, from: arch-api, to: arch-db, arrow: true}
  - {kind: connector, from: arch-jobs, to: arch-db, arrow: true}
  - {kind: text, x: 0.72, y: 0.9, w: 0.28, h: 0.1, text: この色の枠は外部監査の対象, textStyle: small, color: danger}
```

# 導入の流れ

```shapes
aspect: 4
items:
  - {kind: text, x: 0, y: 0, w: 0.6, h: 0.14, text: 利用者が Kumo を使い始めるまでの5段階, color: text_muted}
  - {id: flow-1, kind: roundRect, x: 0, y: 0.22, w: 0.16, h: 0.4, text: 1. 申し込み, textStyle: body, fill: card_alt}
  - {id: flow-2, kind: roundRect, x: 0.21, y: 0.22, w: 0.16, h: 0.4, text: 2. 環境の確認, textStyle: body, fill: card_alt}
  - {id: flow-3, kind: roundRect, x: 0.42, y: 0.22, w: 0.16, h: 0.4, text: 3. 導入, textStyle: body, fill: accent, textColor: on_conditional}
  - {id: flow-4, kind: roundRect, x: 0.63, y: 0.22, w: 0.16, h: 0.4, text: 4. 試験運用, textStyle: body, fill: card_alt}
  - {id: flow-5, kind: roundRect, x: 0.84, y: 0.22, w: 0.16, h: 0.4, text: 5. 本番, textStyle: body, line: support}
  - {kind: connector, from: flow-1, to: flow-2, arrow: true}
  - {kind: connector, from: flow-2, to: flow-3, arrow: true}
  - {kind: connector, from: flow-3, to: flow-4, arrow: true}
  - {kind: connector, from: flow-4, to: flow-5, arrow: true}
  - {kind: text, x: 0, y: 0.7, w: 0.16, h: 0.3, text: [窓口で受付, 1営業日], textStyle: small, color: text_muted}
  - {kind: text, x: 0.21, y: 0.7, w: 0.16, h: 0.3, text: [OS・メモリを, 自動で点検], textStyle: small, color: text_muted}
  - {kind: text, x: 0.42, y: 0.7, w: 0.16, h: 0.3, text: ["**最も時間がかかる**", 平均5営業日], textStyle: small}
  - {kind: text, x: 0.63, y: 0.7, w: 0.16, h: 0.3, text: [2週間、, 実データの一部で], textStyle: small, color: text_muted}
  - {kind: text, x: 0.84, y: 0.7, w: 0.16, h: 0.3, text: [保守契約を, 開始], textStyle: small, color: text_muted}
```

- 申し込みから本番まで、いまは **平均4週間**
- 導入(3)を2営業日に縮めれば、限定公開の10社を3月中に受け入れられる

:::note
導入(3)を短くするのが限定公開までの最大の課題
:::

# 立ち上げの体制

```shapes
aspect: 2.6
items:
  - {id: org-lead, kind: rect, x: 0.4, y: 0, w: 0.2, h: 0.18, text: [責任者, 架空 太郎], textStyle: body, fill: accent, textColor: on_conditional}
  - {id: org-dev, kind: rect, x: 0.05, y: 0.38, w: 0.2, h: 0.18, text: 開発, textStyle: body, fill: card_alt}
  - {id: org-ops, kind: rect, x: 0.4, y: 0.38, w: 0.2, h: 0.18, text: 運用, textStyle: body, fill: card_alt}
  - {id: org-biz, kind: rect, x: 0.75, y: 0.38, w: 0.2, h: 0.18, text: 営業, textStyle: body, fill: card_alt}
  - {id: org-dev-a, kind: roundRect, x: 0.01, y: 0.76, w: 0.13, h: 0.18, text: [基盤, 4名], line: grid}
  - {id: org-dev-b, kind: roundRect, x: 0.16, y: 0.76, w: 0.13, h: 0.18, text: [画面, 3名], line: grid}
  - {id: org-ops-a, kind: roundRect, x: 0.36, y: 0.76, w: 0.13, h: 0.18, text: [導入支援, 2名], line: conditional}
  - {id: org-ops-b, kind: roundRect, x: 0.51, y: 0.76, w: 0.13, h: 0.18, text: [監視, 2名], line: grid}
  - {id: org-biz-a, kind: roundRect, x: 0.71, y: 0.76, w: 0.13, h: 0.18, text: [提案, 2名], line: grid}
  - {id: org-biz-b, kind: roundRect, x: 0.86, y: 0.76, w: 0.13, h: 0.18, text: [契約, 1名], line: grid}
  - {kind: connector, from: org-lead, to: org-dev}
  - {kind: connector, from: org-lead, to: org-ops}
  - {kind: connector, from: org-lead, to: org-biz}
  - {kind: connector, from: org-dev, to: org-dev-a}
  - {kind: connector, from: org-dev, to: org-dev-b}
  - {kind: connector, from: org-ops, to: org-ops-a}
  - {kind: connector, from: org-ops, to: org-ops-b}
  - {kind: connector, from: org-biz, to: org-biz-a}
  - {kind: connector, from: org-biz, to: org-biz-b}
```

:::note
枠で強調した導入支援は、限定公開までに2名を増やす
:::

# 見込み客のファネル

```shapes
aspect: 2.6
items:
  - {kind: rect, x: 0, y: 0, w: 0.6, h: 0.17, text: 説明会の参加 400社, textStyle: body, fill: card_alt}
  - {kind: rect, x: 0.05, y: 0.2, w: 0.5, h: 0.17, text: 資料の請求 180社, textStyle: body, fill: card_alt}
  - {kind: rect, x: 0.1, y: 0.4, w: 0.4, h: 0.17, text: 環境の確認 60社, textStyle: body, fill: card_alt}
  - {kind: rect, x: 0.15, y: 0.6, w: 0.3, h: 0.17, text: 試験運用 24社, textStyle: body, line: conditional}
  - {kind: rect, x: 0.2, y: 0.8, w: 0.2, h: 0.17, text: 契約 10社, textStyle: body, fill: accent, textColor: on_conditional}
  - {kind: text, x: 0.66, y: 0.02, w: 0.34, h: 0.15, text: 展示会と紹介が主な入口, textStyle: small, color: text_muted}
  - {kind: text, x: 0.66, y: 0.42, w: 0.34, h: 0.15, text: "環境の確認で **3社に2社** が脱落", textStyle: small}
  - {kind: text, x: 0.66, y: 0.82, w: 0.34, h: 0.15, text: 限定公開の目標は10社, textStyle: small, color: text_muted}
```

# 配布の形の比較

```shapes
aspect: 2.4
items:
  - {kind: rect, x: 0.27, y: 0, w: 0.21, h: 0.12, text: コンテナ, textStyle: body, fill: card}
  - {kind: rect, x: 0.5, y: 0, w: 0.21, h: 0.12, text: 仮想マシン, textStyle: body, fill: card}
  - {kind: rect, x: 0.73, y: 0, w: 0.21, h: 0.12, text: 専用の機器, textStyle: body, fill: card}
  - {kind: rect, x: 0, y: 0.14, w: 0.25, h: 0.16, text: 導入の速さ, textStyle: body, fill: card}
  - {kind: rect, x: 0.27, y: 0.14, w: 0.21, h: 0.16, text: 速い(1日), line: support}
  - {kind: rect, x: 0.5, y: 0.14, w: 0.21, h: 0.16, text: 中(3日), line: conditional}
  - {kind: rect, x: 0.73, y: 0.14, w: 0.21, h: 0.16, text: 遅い(2週間), line: danger}
  - {kind: rect, x: 0, y: 0.33, w: 0.25, h: 0.16, text: 利用者の負担, textStyle: body, fill: card}
  - {kind: rect, x: 0.27, y: 0.33, w: 0.21, h: 0.16, text: 運用の知識が要る, line: conditional}
  - {kind: rect, x: 0.5, y: 0.33, w: 0.21, h: 0.16, text: 中, line: conditional}
  - {kind: rect, x: 0.73, y: 0.33, w: 0.21, h: 0.16, text: 小さい, line: support}
  - {kind: rect, x: 0, y: 0.52, w: 0.25, h: 0.16, text: 監査の通しやすさ, textStyle: body, fill: card}
  - {kind: rect, x: 0.27, y: 0.52, w: 0.21, h: 0.16, text: 要確認, line: conditional}
  - {kind: rect, x: 0.5, y: 0.52, w: 0.21, h: 0.16, text: 通しやすい, line: support}
  - {kind: rect, x: 0.73, y: 0.52, w: 0.21, h: 0.16, text: 通しやすい, line: support}
  - {kind: rect, x: 0, y: 0.71, w: 0.25, h: 0.16, text: 原価, textStyle: body, fill: card}
  - {kind: rect, x: 0.27, y: 0.71, w: 0.21, h: 0.16, text: 低い, line: support}
  - {kind: rect, x: 0.5, y: 0.71, w: 0.21, h: 0.16, text: 中, line: conditional}
  - {kind: rect, x: 0.73, y: 0.71, w: 0.21, h: 0.16, text: 高い, line: danger}
  - {kind: rect, x: 0.27, y: 0.92, w: 0.03, h: 0.07, fill: support}
  - {kind: text, x: 0.31, y: 0.92, w: 0.1, h: 0.08, text: 良い, textStyle: small, color: text_muted}
  - {kind: rect, x: 0.43, y: 0.92, w: 0.03, h: 0.07, fill: conditional}
  - {kind: text, x: 0.47, y: 0.92, w: 0.1, h: 0.08, text: 条件つき, textStyle: small, color: text_muted}
  - {kind: rect, x: 0.59, y: 0.92, w: 0.03, h: 0.07, fill: danger}
  - {kind: text, x: 0.63, y: 0.92, w: 0.1, h: 0.08, text: 悪い, textStyle: small, color: text_muted}
```

# 改善のサイクル

```shapes
aspect: 2.6
items:
  - {id: cyc-measure, kind: ellipse, x: 0.42, y: 0, w: 0.16, h: 0.28, text: 測る, textStyle: body, fill: card_alt}
  - {id: cyc-find, kind: ellipse, x: 0.66, y: 0.36, w: 0.16, h: 0.28, text: 見つける, textStyle: body, fill: card_alt}
  - {id: cyc-fix, kind: ellipse, x: 0.42, y: 0.72, w: 0.16, h: 0.28, text: 直す, textStyle: body, fill: accent, textColor: on_conditional}
  - {id: cyc-share, kind: ellipse, x: 0.18, y: 0.36, w: 0.16, h: 0.28, text: 伝える, textStyle: body, fill: card_alt}
  - {kind: connector, from: cyc-measure, to: cyc-find, arrow: true}
  - {kind: connector, from: cyc-find, to: cyc-fix, arrow: true}
  - {kind: connector, from: cyc-fix, to: cyc-share, arrow: true}
  - {kind: connector, from: cyc-share, to: cyc-measure, arrow: true}
  - {kind: text, x: 0.38, y: 0.42, w: 0.24, h: 0.16, text: 2週間で1周, textStyle: small, color: text_muted}
  - {kind: text, x: 0.85, y: 0.05, w: 0.15, h: 0.3, text: [試験運用の, 利用者と一緒に回す], textStyle: small, color: text_muted}
```

# 立ち上げまでの計画

```gantt
span: 2026-12 .. 2028-03
ticks:
  - {at: 2026-12, label: 現在}
  - {at: 2027-04, label: 限定公開, status: conditional}
  - {at: 2027-10, label: 一般公開, status: support}
milestones:
  - {at: 2027-07, label: 外部監査}
legend:
  support: 予定どおり
  conditional: 要調整
  danger: 遅れの恐れ
  risk: 人手が足りない期間
groups:
  - id: plan-dev
    label: [開発, 7名]
    rows:
      - label: 基盤
        segments:
          - {from: 2026-12, to: 2027-04, status: support, label: 限定公開版}
          - {from: 2027-04, to: 2027-10, status: support, label: 一般公開版}
      - label: 画面
        segments:
          - {from: 2026-12, to: 2027-05, status: conditional, label: 設計のやり直し}
          - {from: 2027-05, to: 2027-10, status: support, label: 仕上げ}
  - id: plan-ops
    label: [運用, 4名]
    risk: 2027-03 .. 2027-06
    rows:
      - label: 導入支援
        segments:
          - {from: 2027-02, to: 2027-10, status: danger, label: 手作業が多い}
          - {from: 2027-10, to: 2028-03, status: support, label: 自動化後, openEnd: true}
      - id: plan-audit
        label: 監査対応
        segments:
          - {from: 2027-03, to: 2027-07, status: conditional, label: 準備}
annotations:
  - at: 2027-04
    target: plan-ops
    status: danger
    emphasis: true
    text: [限定公開の時点で導入支援が, 2名不足(デモ)]
  - at: 2027-07
    target: plan-audit
    status: conditional
    text: [監査に落ちたら一般公開を, 3か月延ばす(デモ)]
```

- 山場は、導入支援の人手が足りない **2027年3〜6月**

# 判断してほしいこと

:::::compare
::::option 予定どおり 2027年10月に公開
:::pros
- 競合より半年早く出せる
:::
:::cons
- 導入支援の人手を外から借りる費用が要る
:::
::::
::::option 2028年1月に延ばす
:::pros
- 導入の自動化を終えてから公開できる
:::
:::cons
- 限定公開の利用者を3か月待たせる
:::
::::
:::::

:::note
どちらでも外部監査の日程は変えない
:::
