# 断られるべき IR の例

どれも `../demo-deck.json` を1か所だけ変えたもの。`tools/validate_ir.py --self-test` は、これらが全部断られることを確かめる。

| ファイル | 変えたところ |
|---|---|
| `align-on-note.json` | 注記に揃えの指定を足した |
| `bad-variant-name.json` | 変種の名前の形が不正 |
| `color-on-segment.json` | Gantt の区間に16進の色を足した |
| `color-on-slide.json` | スライドに色を足した |
| `design-role-as-status.json` | 状態にデザイン側の役割名(text_primary)を書いた |
| `hex-as-status.json` | 状態に役割名ではなく色を書いた |
| `inline-bold.json` | 文中に見た目の装飾(bold)を書いた |
| `position-on-annotation.json` | 注釈に位置を足した |
| `unknown-subsystem.json` | 登録されていないサブシステム |
