import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  // Route Handler（src/app/api/**/route.ts）は tsconfig.json の paths と同じ
  // `@/*` で src/lib を import している。テストランナー側にも同じ対応づけが
  // 無いと、**Route Handler を単体で import できない** ——
  // `docs/specs/bff-auth-termination.md` AC-11-12「11-2 が作る Route Handler
  // は、いずれも入口でレート制限を通ること。例外の経路を作らない」を検査する
  // には、Route Handler を実際に呼ぶ必要がある。
  //
  // これはテストランナーの解決規則であって実装ではない。tsconfig.json の
  // paths を写しているだけであり、**新しい層・新しい階層規則を作らない**
  // （AC-7-4）。
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  // AC-3-2: テストが0件のとき緑にしない。0件でも成功扱いにするオプションは
  // 追加せず、既定値のままにすることで満たす（このオブジェクトに足さない）。
  test: {
    environment: "node",
  },
});
