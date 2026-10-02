// design-sync 用: .design-sync/tailwind.css を Tailwind v4（apps/web の devDependency）で
// コンパイルし、apps/web/.ds-css/ds.css に書き出す。config.json の cssEntry がこれを指す。
// 実行: node .design-sync/build-css.mjs（リポジトリのルートから）
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const require = createRequire(resolve(root, "apps/web/package.json"));
const postcss = require("postcss");
const tailwind = require("@tailwindcss/postcss");

const from = resolve(here, "tailwind.css");
const to = resolve(root, "apps/web/.ds-css/ds.css");
const result = await postcss([tailwind({ base: here })]).process(readFileSync(from, "utf8"), { from, to });
mkdirSync(dirname(to), { recursive: true });
writeFileSync(to, result.css);
console.log(`wrote ${to} (${result.css.length} bytes)`);

const typesDir = resolve(root, "apps/web/.ds-css/types");
rmSync(typesDir, { recursive: true, force: true });
execFileSync(process.execPath, [require.resolve("typescript/bin/tsc"), "-p", resolve(here, "tsconfig.dts.json")], { stdio: "inherit" });
// 変換ツールは「パッケージの package.json の types」から .d.ts を読む。apps/web は配布用の
// パッケージではないので、生成物の置き場（.ds-css/）を変換ツール専用の小さなパッケージにする。
const outDir = resolve(root, "apps/web/.ds-css");
const appPkg = JSON.parse(readFileSync(resolve(root, "apps/web/package.json"), "utf8"));
const componentFiles = readdirSync(resolve(typesDir, "components")).filter((f) => f.endsWith(".d.ts")).sort();
writeFileSync(
  resolve(typesDir, "index.d.ts"),
  componentFiles.map((f) => `export * from "./components/${f.replace(/\.d\.ts$/, "")}";`).join("\n") + "\n",
);
writeFileSync(
  resolve(outDir, "package.json"),
  JSON.stringify({ name: appPkg.name, version: appPkg.version, private: true, types: "types/index.d.ts" }, null, 2) + "\n",
);
console.log(`wrote ${typesDir} (${componentFiles.length} components) and ${resolve(outDir, "package.json")}`);
