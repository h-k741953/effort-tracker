import { describe, it, expect } from "vitest";
import config from "../next.config";

// AC-1-9 / P-6: OpenNext（ADR 0013）は静的エクスポート（output: 'export'）と
// 非互換。この設定が壊れて 'export' に戻ると、この Vitest が Red を踏む。
describe("next.config", () => {
  it("does not use static export (ADR 0013 / P-6)", () => {
    expect(config.output).not.toBe("export");
  });
});
