# 出所の記録 — Zen Kaku Gothic New

| 項目 | 内容 |
|---|---|
| 取得元 | npm パッケージ `@fontsource/zen-kaku-gothic-new` |
| 版 | 5.3.1 |
| 取得方法 | `npm pack`（依存には足していない。`woff2` とライセンス文だけを取り出して置いた） |
| tarball の sha256 | `4be769463585cf231ace87d2da71ea45ab3ce8223f59700405a7b9ac403b6028` |
| ライセンス | SIL Open Font License 1.1（OFL-1.1）。全文は同じディレクトリの `LICENSE`（パッケージの `LICENSE` をそのまま写した） |
| ウェイト | 400 / 500 / 700（斜体なし。`docs/specs/design-system.md` 4-3-b） |
| 使ったサブセット | パッケージの `unicode.json` に `unicode-range` が定義された分割のうち、番号スライス `[2]`〜`[119]`、`latin`、`latin-ext`。`cyrillic` は使わない。`unicode.json` に範囲の定義が無い `japanese` 一括ファイルは使わない |
| `unicode-range` の値 | `layout.tsx` の各 `localFont` 呼び出しの `declarations` に、`unicode.json` の値をそのまま写した（パッケージの CSS の値と一致することを取り込み時に確認した） |
| ファイル名 | パッケージの `files/` の名前のまま（`zen-kaku-gothic-new-<サブセット>-<ウェイト>-normal.woff2`） |
