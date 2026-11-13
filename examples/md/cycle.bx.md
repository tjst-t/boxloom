---
title: 循環の図の見本
subtitle: 輪に並んだ段(すべて架空)
author: Boxloom デモ
template: report
---

# 拠点は5つの段を回して育てる

:::lead
架空の町 Kirinoha の地域拠点は、0〜4 の段を1年ごとに回す
:::

```cycle
start: 0
steps:
  - {title: 場を知る, detail: 住民の声を集める}
  - {title: 仲間を集める, detail: 担い手を3人以上}
  - {title: 小さく試す, detail: 月1回の催し, emphasis: true}
  - {title: 形にする, detail: 運営の決まりを作る, status: conditional}
  - {title: ふり返る, detail: 次の年へ渡す}
legend:
  conditional: 町の予算が付くときだけ
```

# 段ごとに、やることと見るものを決める

:::lead
左の輪の段ごとに、右に要点を書く
:::

::::columns
:::column
```cycle
start: 0
steps:
  - title: 場を知る
  - title: 仲間を集める
  - title: 小さく試す
  - title: 形にする
  - title: ふり返る
```
:::
:::column 段ごとの要点
- **0 場を知る**:住民 30 人に話を聞く
- **1 仲間を集める**:担い手を3人以上決める
- **2 小さく試す**:月1回の催しを半年続ける
- **3 形にする**:運営の決まりを文書にする
- **4 ふり返る**:数と声をまとめ、次の年へ渡す
:::
::::
