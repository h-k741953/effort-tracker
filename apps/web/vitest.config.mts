import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // AC-3-2: テストが0件のとき緑にしない。既定値のまま変更しない。
    environment: "node",
  },
});
