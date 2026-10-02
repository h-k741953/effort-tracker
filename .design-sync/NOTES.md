# design-sync メモ（effort-tracker）

## このリポジトリの形

- 配布用のパッケージではなく Next.js アプリ（`apps/web`）。Storybook は無い（package の形）。
- 変換ツールは「パッケージの package.json」を起点に動くため、`node .design-sync/build-inputs.mjs` で
  `apps/web/.ds-css/`（git 管理外）を変換ツール専用の小さなパッケージとして組み立てる。
  - `ds.css`: `.design-sync/tailwind.css` を Tailwind v4 でコンパイルしたもの（config の `cssEntry`）
  - `types/`: tsc で生成した共通コンポーネントの型宣言（`.design-sync/tsconfig.dts.json`）
  - `package.json`: `types` が `types/index.d.ts` を指す
- 変換の実行（リポジトリのルートから）:
  ```sh
  node .design-sync/build-inputs.mjs
  node .ds-sync/resync.mjs --config .design-sync/config.json --node-modules apps/web/node_modules \
    --entry apps/web/.ds-css/dist/index.js --out ./ds-bundle --no-render-check [--remote .design-sync/.cache/remote-sync.json]
  ```
  `--entry` は存在しないパスでよい（PKG_DIR を `apps/web/.ds-css` に決めるためだけに渡す。入口は `srcDir` から合成される）。

## はまりどころ（解決済み）

- `srcDir` を指定しないと `src/` 全体（`app/layout.tsx` → `globals.css` の `@import "tailwindcss"`）を取り込もうとして esbuild が失敗する。`srcDir` は `../src/components`。
- 型宣言を生成しないと `<Name>.d.ts` の props が `[key: string]: unknown` になる。変換ツールはドットで始まるディレクトリを走査しないので、`package.json` の `types` で場所を示している。
- `@source` で `components/` を走査すると、テストファイル（禁止表現を例として持つ）から `rounded-full` などが CSS に混ざる。`@source not` でテストを外している。
- `lib/role-cookie.ts` の Node の型を解決するため、`tsconfig.dts.json` で `typeRoots` を `apps/web/node_modules/@types` に向けている。

## 表示の確認

- ファイアウォール（`init-firewall.sh`）が playwright の配布元を通さず、chromium を入れられなかった。
  2026-10-02 の初回同期は `--no-render-check` で行い、**機械による表示確認・採点はしていない**。
  人間がローカルの確認ページ（`node .ds-sync/storybook/http-serve.mjs ./ds-bundle` → `/.review.html`）で確かめたうえで、全件のアップロードを指示した。
  `report_validate` に送った件数（bad 0）は、レンダーチェックの実測ではない。
- resync の capture 段はブラウザが要るため失敗する（build / diff / validate は通る）。ブラウザを入れられるまでは想定どおり。

## Known render warns

- `[RENDER_SKIPPED]`（上記の理由。ブラウザが入るまで毎回出る）

## Re-sync risks（次回の同期で古くなりうるもの）

- `config.json` の `dtsPropsFor.RoleSwitcher` は `Role` の union（`"Engineer" | "Approver"`）の写し。`apps/web/src/lib/role-cookie.ts` の `Role` を変えたら合わせて直す。
- `.design-sync/tailwind.css` の `@source inline(...)` は、トークン（AC-2 の17個）と、デザインエージェント向けの配置クラスの手書きの一覧。トークンを増減したら直す。`conventions.md` の表も同じ。
- `.design-sync/previews/*.tsx` は AC-6 の props に依存する。props を変えたらプレビューも直す。
- プレビューは機械で採点していない（上記）。ブラウザが使えるようになったら、`package-capture.mjs` で採点し直す。
