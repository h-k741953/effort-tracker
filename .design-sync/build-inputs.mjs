// design-sync 用: .design-sync/tailwind.css を Tailwind v4（apps/web の devDependency）で
// コンパイルし、apps/web/.ds-css/ds.css に書き出す。config.json の cssEntry がこれを指す。
// 実行: node .design-sync/build-inputs.mjs（リポジトリのルートから）
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

// フォント: アプリは next/font/local（apps/web/src/app/layout.tsx）で同梱フォントを読むため、
// CSS に @font-face が無い。Claude Design のデザインでも同じ書体で描かれるよう、layout.tsx の
// localFont 呼び出し（src の path・weight と declarations の font-family・unicode-range）から
// @font-face を生成し、apps/web/.ds-css/fonts.css に書き出す。config.json の extraFonts がこれを指す。
// layout.tsx から機械的に作るので、範囲やウェイトを変えても手で直す箇所は無い。
const layoutSrc = readFileSync(resolve(root, "apps/web/src/app/layout.tsx"), "utf8");
const fontFaces = [];
for (const call of layoutSrc.split("localFont({").slice(1)) {
  const body = call.slice(0, call.indexOf("});"));
  const family = /prop:\s*"font-family",\s*value:\s*"([^"]+)"/.exec(body)?.[1];
  const range = /prop:\s*"unicode-range",\s*value:\s*"([^"]+)"/.exec(body)?.[1];
  const srcs = [...body.matchAll(/path:\s*"\.\/([^"]+)",\s*weight:\s*"(\d+)"/g)];
  if (!family || !range || srcs.length === 0) throw new Error(`layout.tsx の localFont 呼び出しを読めない: ${body.slice(0, 120)}`);
  for (const [, path, weight] of srcs) {
    fontFaces.push(
      `@font-face {\n  font-family: "${family}";\n  src: url("../src/app/${path}") format("woff2");\n  font-weight: ${weight};\n  font-style: normal;\n  font-display: swap;\n  unicode-range: ${range};\n}\n`,
    );
  }
}
writeFileSync(resolve(outDir, "fonts.css"), fontFaces.join(""));
console.log(`wrote ${resolve(outDir, "fonts.css")} (${fontFaces.length} @font-face)`);
