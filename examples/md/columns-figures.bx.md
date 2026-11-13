---
title: 列の中の図の見本
subtitle: 対の並び・図・写真の並びを、要点の横に置く(すべて架空)
author: Boxloom デモ
template: report
---

# 聞き取りの論点ごとに打ち手を決めた

:::lead
架空の町 Kirinoha の図書館は、利用者 12 人の聞き取りから3つの論点を抜き出した
:::

:::::::columns
::::::column 抜き出した論点
- 返却の半分は閉館後に集まる
- 雨の日は返却箱の本が濡れる
- 返却したのに延滞の知らせが届く
::::::
::::::column 論点と打ち手 {role=main}
:::::pairs 分かったこと | 打ち手
:::subject
返却は閉館後の 20 時から 23 時に集まる
:::
:::response
- 返却箱を2つに増やす
- 箱が満ちたら翌朝の当番に知らせる
:::
:::subject
雨の日は箱の口から本が濡れる
:::
:::response
口に庇を付け、濡れた本は乾かしてから棚に戻す
:::
::::subject
:::quote 利用者 B、聞き取りにて
返したのに延滞の知らせが来て驚いた
:::
::::
:::response
箱の中身は開館と同時に読み取り、前の晩の日付で記録する
:::
:::::
::::::
:::::::

# 再編は3つの段で1年かけて進める

::::columns
:::column 段ごとの要点
- 4月〜6月は、乗降を数えて住民に話を聞く
- 7月〜9月は、路線の案と3年分の費用を作る
- 10月〜12月は、報告書にまとめて議会に諮る
- どの段も、終わりに住民の会で中身を見せる
- 費用の見積もりは、国の補助の額が決まってから直す
:::
:::column {role=main}
```phases
steps:
  - title: 現状を知る
    period: 4月〜6月
    items:
      - title: 乗降の調べ
        detail: 全 42 路線を1週間
      - title: 住民への聞き取り
        detail: 沿線の 6 地区で
  - title: 案を作る
    period: 7月〜9月
    items:
      - title: 路線の案
        detail: 幹線と支線に分ける
      - title: 費用の見積もり
        detail: 3 年分の運行費
        status: conditional
  - title: まとめる
    period: 10月〜12月
    items:
      - title: 報告書
        detail: 議会に諮る
legend:
  conditional: 国の補助の額で変わる
```
:::
::::

# 案 B だけが費用と期限の両方を満たす

::::columns
:::column 読み取り {role=main}
- 案 A は費用がかからないが、保守が 2028 年に切れる
- 案 B は年に約 200 万円かかるが、2034 年まで保守が続く
- 止まる時間は、どちらの案も夜間に収まる
- 次の年の予算で案 B を求める
:::
:::column
```matrix
columns: [案 A(据え置き), 案 B(段階移行)]
rows:
  - label: 費用
    cells:
      - {status: support, text: 追加なし}
      - {status: conditional, text: 年に約200万円}
  - label: 保守の期限
    cells:
      - {status: danger, text: 2028年に切れる}
      - {status: support, text: 2034年まで}
  - label: 止まる時間
    cells:
      - {status: support, text: なし}
      - {status: support, text: 夜間に1時間ずつ}
legend:
  support: 満たす
  danger: 満たさない
```
:::
::::

# 移行は2つのチームと窓口で担う

::::columns
:::column 体制の要点 {role=main}
- 責任者の下に、基盤と移行の2つのチームを置く
- 認証の担当が1名足りないので、4月までに採る
- 窓口は移行の間だけ、平日の夜も開ける
:::
:::column
```orgchart
root: {title: 推進の責任者, detail: 架空 花子}
units:
  - title: 基盤チーム
    detail: 3名
    members:
      - {title: ネットワーク, detail: 2名}
      - {title: 認証, detail: 1名, status: danger}
  - title: 移行チーム
    detail: 2名
legend:
  danger: 担当が足りない
```
:::
::::

# 登録は3つの画面で終わる

::::columns
:::column 画面の流れ
1. 一つ目の画面で、利用者の番号を読み取る
2. 二つ目の画面で、借りる本を読み取る
3. 三つ目の画面で、返す日を確かめて終える
:::
:::column 試作の画面 {role=main}
![一つ目の画面:番号の読み取り](images/step-1.png) ![二つ目の画面:本の読み取り](images/step-2.png) ![三つ目の画面:返す日の確認](images/step-3.jpg)
:::
::::
