# 出所の記録 — Space Grotesk

| 項目 | 内容 |
|---|---|
| 取得元 | npm パッケージ `@fontsource/space-grotesk` |
| 版 | 5.3.0 |
| 取得方法 | `npm pack`（依存には足していない。`woff2` とライセンス文だけを取り出して置いた） |
| tarball の sha256 | `2adb650694b6387b1862fee8b086a3a47ce9f32c10f20b8ec109aa1fcdfd07b7` |
| ライセンス | SIL Open Font License 1.1（OFL-1.1）。全文は同じディレクトリの `LICENSE`（パッケージの `LICENSE` をそのまま写した） |
| ウェイト | 500 / 600 / 700（斜体なし。`docs/specs/design-system.md` 4-3-b） |
| 使ったサブセット | `latin`、`latin-ext`。`vietnamese` は使わない |
| `unicode-range` の値 | `layout.tsx` の各 `localFont` 呼び出しの `declarations` に、`unicode.json` の値をそのまま写した（パッケージの CSS の値と一致することを取り込み時に確認した） |
| ファイル名 | パッケージの `files/` の名前のまま（`space-grotesk-<サブセット>-<ウェイト>-normal.woff2`） |
