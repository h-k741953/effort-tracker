// design-sync 用: SideNav が使う next/link は、読み込みの時点で process.env.__NEXT_* を参照する。
// 変換ツールのバンドルが置き換えるのは process.env.NODE_ENV だけなので、ブラウザ（Claude Design の
// プレビューとデザイン）では「process is not defined」でバンドル全体が止まる。
// config.json の extraEntries から部品より先に評価させ、process が無いときだけ空の env を用意する。
// 何も export しない。アプリ（apps/web）の実行には関わらない。
if (typeof globalThis.process === "undefined") {
  globalThis.process = { env: {} };
}
