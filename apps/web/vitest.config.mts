import { defineConfig } from "vitest/config";

export default defineConfig({
  // AC-3-2: テストが0件のとき緑にしない。0件でも成功扱いにするオプションは
  // 追加せず、既定値のままにすることで満たす（このオブジェクトに足さない）。
  test: {
    environment: "node",
  },
});
